package keyboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	cfg "github.com/xrfr/t-rex/v2/config"
	"github.com/xrfr/t-rex/v2/protocol"
)

//go:embed words.txt
var wordsFile embed.FS

type Keyboard struct {
	keyboard   [][]string
	model      KeyboardModel
	priorModel KeyboardModel // saved when switching to voice mode
	decoder    *json.Decoder
	decoderMu  sync.Mutex
	encoder    *json.Encoder
	width      int
	// connections kept for cleanup
	inputConn  net.Conn
	outputConn net.Conn
	// autocomplete
	dictionary  *Trie
	currentWord string
	suggestions [4]string
	// display
	compact      bool
	keyboardView string
	handCols     int
	rowOffsets   []int
	minKeyWidth  int
	paneHeight   int
	// voice mode
	sttMode string // "parakeet", "sherpa", or "companion"
	// when true, voice/text toggles are written back to config so the next
	// keyboard launch resumes in the same mode
	stickyVoice bool
}

type RuntimeError error

func (k *Keyboard) getKeyboardModel(mode string) KeyboardModel {
	switch mode {
	case "simple":
		return NewKeyboardModelSimple(k.keyboard)
	case "front-chorded":
		return NewKeyboardModelFrontChorded(k.keyboard)
	case "voice":
		return k.createVoiceModel()
	default:
		slog.Warn("unknown keyboard mode, falling back to simple", "mode", mode)
		return NewKeyboardModelSimple(k.keyboard)
	}
}

func (k *Keyboard) createVoiceModel() *VoiceModel {
	// Re-read config so Settings app changes take effect on next voice entry
	// without needing a rex restart.
	sttMode := k.sttMode
	if fresh, err := cfg.LoadConfig(); err == nil {
		sttMode = cfg.NormalizeSTTMode(fresh.STTMode)
	}
	k.sttMode = sttMode

	// STT always runs server-side (resident model in the daemon, or the iOS
	// companion app); the keyboard just sends voice commands and waits for text.
	provider := NewRemoteSTTProvider(func(action string) error {
		if k.encoder == nil {
			return fmt.Errorf("not connected to server")
		}
		msg := protocol.VoiceCommandMessage{
			BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeVoiceCommand},
			Action:      action,
		}
		return k.encoder.Encode(msg)
	})
	return NewVoiceModel(provider, sttMode == cfg.STTModeCompanion)
}

func NewKeyboard(keyboardMode string, dictWords int, compact bool, autocomplete bool, sttMode string, defaultVoice bool, stickyVoice bool, keyboardView string, keyboardLayout string) *Keyboard {
	layout := keyboardLayoutForName(keyboardLayout)
	k := &Keyboard{
		keyboard:     layout.keys,
		compact:      compact,
		keyboardView: cfg.NormalizeKeyboardView(keyboardView),
		handCols:     layout.handCols,
		rowOffsets:   layout.rowOffsets,
		minKeyWidth:  layout.minKeyWidth,
		paneHeight:   cfg.KeyboardPaneHeightForLayout(compact, autocomplete, keyboardLayout),
		sttMode:      sttMode,
		stickyVoice:  stickyVoice,
	}
	if defaultVoice {
		// Start in voice mode; save the normal model so Fn can toggle back
		k.priorModel = k.getKeyboardModel(keyboardMode)
		k.model = k.createVoiceModel()
	} else {
		k.model = k.getKeyboardModel(keyboardMode)
	}

	if autocomplete {
		f, err := wordsFile.Open("words.txt")
		if err == nil {
			defer f.Close()
			k.dictionary = LoadTrie(f, dictWords)
		}
	}

	return k
}

func (k *Keyboard) Run() error {
	// close connections on exit
	defer func() {
		if k.inputConn != nil {
			k.inputConn.Close()
		}
		if k.outputConn != nil {
			k.outputConn.Close()
		}
	}()

	// Connect before starting the tea program so connection failures surface
	// as a plain error rather than a panic from inside the bubble tea loop.
	inputConn, err := net.Dial("unix", protocol.InputSocketPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "rex keyboard: could not connect to server (is the rex server running?)\n  %v\n", err)
		return err
	}
	k.inputConn = inputConn
	k.decoder = json.NewDecoder(inputConn)

	outputConn, err := net.Dial("unix", protocol.OutputSocketPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "rex keyboard: could not connect to server (is the rex server running?)\n  %v\n", err)
		return err
	}
	k.outputConn = outputConn
	k.encoder = json.NewEncoder(outputConn)

	slog.Info("connected to server", "input", protocol.InputSocketPath(), "output", protocol.OutputSocketPath())

	p := tea.NewProgram(k)
	_, err = p.Run()
	return err
}

// implement tea.Model interface Init, Update
var QuitCmd = tea.Sequence(tea.ExitAltScreen, tea.Quit)

func (k *Keyboard) Init() tea.Cmd {
	// 0. return tea.Cmd to read socket
	// 1. ReadSocket returns a message containing the parsed message or an error
	// 2. Update parses message and returns ReadSocket command again
	// 3. back to 1
	cmds := []tea.Cmd{tea.EnterAltScreen}
	// If we started in voice mode (defaultVoice), drain the queued STT init cmd
	// so Initialize() actually runs.
	if vm, ok := k.model.(*VoiceModel); ok {
		if c := vm.DrainCmd(); c != nil {
			cmds = append(cmds, c)
		}
	}
	return tea.Batch(cmds...)
}

func (k *Keyboard) ReadSocket() tea.Msg {
	k.decoderMu.Lock()
	defer k.decoderMu.Unlock()
	msg, err := protocol.ParseMessage(k.decoder)
	if err != nil {
		return RuntimeError(fmt.Errorf("failed to parse message: %w", err))
	}
	return msg
}

func (k *Keyboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// slog.Debug("main loop update", "type", fmt.Sprintf("%T", msg), "msg", msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return k, QuitCmd
		}
	case RuntimeError:
		slog.Error("runtime error", "err", msg)
		return k, QuitCmd
	case tea.WindowSizeMsg:
		k.width = msg.Width
		// NOTE somehow returning read socket here causes an issue
		return k, nil
	case protocol.KeyInputMessage:
		// Intercept Fn for voice mode toggle before passing to model
		if msg.IsKeyDown && msg.KeyInput == protocol.KeyFn {
			initCmd := k.toggleVoiceMode()
			return k, tea.Batch(initCmd, k.ReadSocket)
		}
		// Process the input and send response, then read next message
		return k, tea.Sequence(k.handleDeviceInput(msg), k.ReadSocket)
	case protocol.VoiceTextMessage:
		// Server produced a final transcription (local model or companion app).
		// Deliver to RemoteSTTProvider so Transcribe() unblocks.
		if vm, ok := k.model.(*VoiceModel); ok {
			if remote, ok := vm.STTProvider().(*RemoteSTTProvider); ok {
				select {
				case remote.FinalCh <- msg.Text:
				default:
					slog.Warn("remote stt finalCh full, dropping")
				}
			}
		}
		return k, k.ReadSocket
	case protocol.CompanionStatusMessage:
		// Server reports STT readiness change (model loaded / companion connected).
		// Deliver to RemoteSTTProvider so Initialize() can unblock.
		if vm, ok := k.model.(*VoiceModel); ok {
			if remote, ok := vm.STTProvider().(*RemoteSTTProvider); ok {
				// Drain-and-resend to avoid blocking the Update loop.
				select {
				case remote.ReadyCh <- msg.Connected:
				default:
					select {
					case <-remote.ReadyCh:
					default:
					}
					remote.ReadyCh <- msg.Connected
				}
			}
		}
		return k, k.ReadSocket
	case STTResultMsg:
		if vm, ok := k.model.(*VoiceModel); ok {
			vm.HandleMsg(msg)
			cmd := vm.DrainCmd()
			if msg.Err == nil && strings.TrimSpace(msg.Text) != "" {
				// In companion mode, the server already injected text via smart replace.
				// Don't send PressStringMessage to avoid double injection.
				if k.sttMode != cfg.STTModeCompanion {
					sendCmd := k.sendString(strings.TrimSpace(msg.Text) + ". ")
					if cmd != nil {
						return k, tea.Batch(cmd, sendCmd)
					}
					return k, sendCmd
				}
			}
			if cmd != nil {
				return k, cmd
			}
		}
		return k, nil
	case DeleteTickMsg:
		if vm, ok := k.model.(*VoiceModel); ok {
			return k.deleteRepeat(vm.deleting, msg, vm)
		}
		if sm, ok := k.model.(*KeyboardModelSimple); ok {
			return k.deleteRepeat(sm.deleting, msg, sm)
		}
		return k, nil
	case DPadArrowTickMsg:
		if vm, ok := k.model.(*VoiceModel); ok {
			vm.HandleMsg(msg)
			cmd := vm.DrainCmd()
			pressCmd := k.sendKeyPress(msg.Key)
			if cmd != nil {
				return k, tea.Batch(cmd, pressCmd)
			}
			return k, pressCmd
		}
		return k, nil
	case STTInitDoneMsg, SpinnerTickMsg, ErrorTimeoutMsg:
		if vm, ok := k.model.(*VoiceModel); ok {
			vm.HandleMsg(msg)
			if cmd := vm.DrainCmd(); cmd != nil {
				return k, cmd
			}
		}
		return k, nil
	case DPadHoldTickMsg:
		if sm, ok := k.model.(*KeyboardModelSimple); ok {
			sm.HandleMsg(msg)
			if cmd := sm.DrainCmd(); cmd != nil {
				return k, cmd
			}
		}
		return k, nil
	}

	// For any other message type (including first message from Init), read from socket
	return k, k.ReadSocket
}

// toggleVoiceMode switches between voice and prior keyboard model. When entering
// voice mode it returns the VoiceModel's queued init command so bubbletea actually
// runs STT Initialize() and the spinner tick — without this the model stays stuck
// in stateInitializing forever.
func (k *Keyboard) toggleVoiceMode() tea.Cmd {
	if _, isVoice := k.model.(*VoiceModel); isVoice {
		// Switch back to prior model
		if k.priorModel != nil {
			k.model = k.priorModel
			k.priorModel = nil
			resizeKeyboardPane(k.paneHeight)
			slog.Info("voice mode off")
			k.persistVoiceState(false)
		}
		return nil
	}
	// Switch to voice mode
	k.priorModel = k.model
	vm := k.createVoiceModel()
	k.model = vm
	resizeKeyboardPane(1)
	slog.Info("voice mode on")
	k.persistVoiceState(true)
	return vm.DrainCmd()
}

// persistVoiceState writes the current voice/text mode back to config when
// the user enabled "sticky" in Settings, so the next keyboard launch resumes
// in the same mode. No-op when sticky is off or the value is already correct.
func (k *Keyboard) persistVoiceState(voiceOn bool) {
	if !k.stickyVoice {
		return
	}
	fresh, err := cfg.LoadConfig()
	if err != nil {
		slog.Warn("sticky voice: load config failed", "err", err)
		return
	}
	if fresh.DefaultVoice == voiceOn {
		return
	}
	fresh.DefaultVoice = voiceOn
	if err := cfg.SaveConfig(fresh); err != nil {
		slog.Warn("sticky voice: save config failed", "err", err)
	}
}

// resizeKeyboardPane resizes the keyboard's own pane. The pane is created
// unfocused and can be joined into any window, so index-based targets don't
// work — $TMUX_PANE always names our pane.
func resizeKeyboardPane(height int) {
	pane := os.Getenv("TMUX_PANE")
	if pane == "" {
		return
	}
	exec.Command("tmux", "resize-pane", "-t", pane, "-y", fmt.Sprintf("%d", height)).Run()
}

// handle key input and optionally provide a command to send key press to server
func (k *Keyboard) handleDeviceInput(msg protocol.KeyInputMessage) tea.Cmd {
	// Intercept shoulder buttons for autocomplete selection (only when suggestions exist)
	if msg.IsKeyDown && k.hasSuggestions() {
		if result := k.handleSuggestionSelect(msg.KeyInput); result != nil {
			return k.sendString(*result)
		}
	}

	// Process the key input through the model (both keydown and keyup)
	keyToPress := k.model.HandleKey(msg.KeyInput, msg.IsKeyDown)

	// Voice model: intercept X to exit, B for delete, otherwise drain queued commands
	if vm, ok := k.model.(*VoiceModel); ok {
		if msg.IsKeyDown && msg.KeyInput == protocol.KeyX {
			return k.toggleVoiceMode()
		}
		if msg.IsKeyDown && msg.KeyInput == protocol.KeyA {
			return k.sendKeyPress(string(protocol.SpecialKeyEnter))
		}
		if msg.KeyInput == protocol.KeyB {
			// HandleKey was already called above — it sets deleting state and queues ticks
			cmd := vm.DrainCmd()
			if msg.IsKeyDown {
				// Send immediate first delete + any queued tick cmd
				delCmd := k.sendKeyPress(string(protocol.SpecialKeyDelete))
				if cmd != nil {
					return tea.Batch(cmd, delCmd)
				}
				return delCmd
			}
			return cmd
		}
		// If the voice model returned a key to press (e.g. DPad → arrow keys),
		// forward it to the pane; server's HandleKeyPress passes multi-char
		// tmux key names through verbatim.
		if keyToPress != nil {
			drainCmd := vm.DrainCmd()
			pressCmd := k.sendKeyPress(*keyToPress)
			if drainCmd != nil {
				return tea.Batch(drainCmd, pressCmd)
			}
			return pressCmd
		}
		return vm.DrainCmd()
	}

	// Simple model: B is delete (hold-to-repeat), mirroring voice mode.
	if sm, ok := k.model.(*KeyboardModelSimple); ok && msg.KeyInput == protocol.KeyB {
		cmd := sm.DrainCmd()
		if msg.IsKeyDown {
			delCmd := k.sendKeyPress(string(protocol.SpecialKeyDelete))
			if cmd != nil {
				return tea.Batch(cmd, delCmd)
			}
			return delCmd
		}
		return cmd
	}

	// Simple model queues dpad hold ticks; drain so bubbletea schedules them.
	var simpleCmd tea.Cmd
	if sm, ok := k.model.(*KeyboardModelSimple); ok {
		simpleCmd = sm.DrainCmd()
	}

	if keyToPress == nil {
		return simpleCmd
	}

	// Intercept MIC key to toggle voice mode
	if len(*keyToPress) == 1 && rune((*keyToPress)[0]) == protocol.SpecialKeyMic {
		if simpleCmd != nil {
			return tea.Batch(simpleCmd, k.toggleVoiceMode())
		}
		return k.toggleVoiceMode()
	}

	// Track typed characters for autocomplete
	k.updateCurrentWord(*keyToPress)

	pressCmd := k.sendKeyPress(*keyToPress)
	if simpleCmd != nil {
		return tea.Batch(simpleCmd, pressCmd)
	}
	return pressCmd
}

func (k *Keyboard) sendKeyPress(key string) tea.Cmd {
	return func() tea.Msg {
		message := protocol.PressKeyMessage{
			BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyPress},
			KeyPress:    key,
		}
		if err := k.encoder.Encode(message); err != nil {
			return RuntimeError(fmt.Errorf("failed to encode message: %w", err))
		}
		return nil
	}
}

func (k *Keyboard) sendString(value string) tea.Cmd {
	return func() tea.Msg {
		message := protocol.PressStringMessage{
			BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypePressString},
			Value:       value,
		}
		if err := k.encoder.Encode(message); err != nil {
			return RuntimeError(fmt.Errorf("failed to encode message: %w", err))
		}
		return nil
	}
}

// deleteRepeat sends one delete and re-queues the next repeat tick when the
// model is still in its deleting state. Shared by voice and simple models.
func (k *Keyboard) deleteRepeat(deleting bool, msg tea.Msg, m interface {
	HandleMsg(tea.Msg)
	DrainCmd() tea.Cmd
}) (tea.Model, tea.Cmd) {
	if !deleting {
		return k, nil
	}
	m.HandleMsg(msg)
	cmd := m.DrainCmd()
	delCmd := k.sendKeyPress(string(protocol.SpecialKeyDelete))
	if cmd != nil {
		return k, tea.Batch(cmd, delCmd)
	}
	return k, delCmd
}

func (k *Keyboard) handleSuggestionSelect(key int) *string {
	var idx int
	switch key {
	case protocol.KeyL1:
		idx = 0
	case protocol.KeyL2:
		idx = 1
	case protocol.KeyR1:
		idx = 2
	case protocol.KeyR2:
		idx = 3
	default:
		return nil
	}

	if k.suggestions[idx] == "" {
		return nil
	}

	remaining := k.suggestions[idx][len(k.currentWord):] + " "
	slog.Info("autocomplete", "word", k.suggestions[idx], "remaining", remaining)

	k.currentWord = ""
	k.suggestions = [4]string{}

	return &remaining
}

func (k *Keyboard) updateCurrentWord(typed string) {
	if len(typed) != 1 {
		return
	}

	ch := rune(typed[0])

	switch ch {
	case protocol.SpecialKeySpace, protocol.SpecialKeyEnter, protocol.SpecialKeyClose, protocol.SpecialKeyDelete:
		k.currentWord = ""
		k.suggestions = [4]string{}
		return
	}

	if ch >= 'a' && ch <= 'z' {
		k.currentWord += string(ch)
	} else if ch == '\b' || ch == 127 {
		if len(k.currentWord) > 0 {
			k.currentWord = k.currentWord[:len(k.currentWord)-1]
		}
	} else {
		k.currentWord = ""
		k.suggestions = [4]string{}
		return
	}

	k.updateSuggestions()
}

func (k *Keyboard) hasSuggestions() bool {
	return k.suggestions != [4]string{}
}

func (k *Keyboard) updateSuggestions() {
	k.suggestions = [4]string{}
	if k.dictionary == nil || k.currentWord == "" {
		return
	}
	results := k.dictionary.Search(k.currentWord, 4)
	for i, w := range results {
		k.suggestions[i] = w
	}
	slog.Debug("suggestions updated", "currentWord", k.currentWord, "suggestions", k.suggestions)
}
