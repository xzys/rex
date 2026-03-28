package keyboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
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
	compact    bool
	paneHeight int
}

type RuntimeError error

func getKeyboardModel(mode string) KeyboardModel {
	switch mode {
	case "chorded":
		return NewKeyboardModelChorded(keyboardLayout, false)
	case "chorded-simultaneous":
		return NewKeyboardModelChorded(keyboardLayout, true)
	case "simple":
		return NewKeyboardModelSimple()
	case "front-chorded":
		return NewKeyboardModelFrontChorded(keyboardLayout)
	default:
		panic("invalid keyboard mode")
	}
}

func NewKeyboard(keyboardMode string, dictWords int, compact bool, autocomplete bool) *Keyboard {
	model := getKeyboardModel(keyboardMode)

	var dictionary *Trie
	if autocomplete {
		f, err := wordsFile.Open("words.txt")
		if err == nil {
			defer f.Close()
			dictionary = LoadTrie(f, dictWords)
		}
	}

	paneHeight := 12
	if compact {
		paneHeight = 8
	}

	return &Keyboard{
		keyboard:   keyboardLayout,
		model:      model,
		width:      0,
		dictionary: dictionary,
		compact:    compact,
		paneHeight: paneHeight,
	}
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

	p := tea.NewProgram(k)
	_, err := p.Run()
	return err
}

// implement tea.Model interface Init, Update
var QuitCmd = tea.Sequence(tea.ExitAltScreen, tea.Quit)

func (k *Keyboard) Init() tea.Cmd {
	// Connect to input socket (receives KeyInput from server)
	inputConn, err := net.Dial("unix", protocol.InputSocketPath())
	if err != nil {
		slog.Error("failed to connect to input socket", "err", err)
		return QuitCmd
	}
	k.inputConn = inputConn
	k.decoder = json.NewDecoder(inputConn)

	// Connect to output socket (sends KeyPress to server)
	outputConn, err := net.Dial("unix", protocol.OutputSocketPath())
	if err != nil {
		slog.Error("failed to connect to output socket", "err", err)
		inputConn.Close()
		return QuitCmd
	}
	k.outputConn = outputConn
	k.encoder = json.NewEncoder(outputConn)

	slog.Info("connected to server", "input", protocol.InputSocketPath(), "output", protocol.OutputSocketPath())

	// 0. return tea.Cmd to read socket
	// 1. ReadSocket returns a message containing the parsed message or an error
	// 2. Update parses message and returns ReadSocket command again
	// 3. back to 1
	return tea.EnterAltScreen
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
		// Process the input and send response, then read next message
		return k, tea.Sequence(k.handleDeviceInput(msg), k.ReadSocket)
	case STTResultMsg:
		if vm, ok := k.model.(*VoiceModel); ok {
			vm.HandleMsg(msg)
			cmd := vm.DrainCmd()
			if msg.Err == nil && strings.TrimSpace(msg.Text) != "" {
				sendCmd := k.sendString(strings.TrimSpace(msg.Text) + ". ")
				if cmd != nil {
					return k, tea.Batch(cmd, sendCmd)
				}
				return k, sendCmd
			}
			if cmd != nil {
				return k, cmd
			}
		}
		return k, nil
	case DeleteTickMsg:
		if vm, ok := k.model.(*VoiceModel); ok {
			if vm.deleting {
				vm.HandleMsg(msg)
				cmd := vm.DrainCmd()
				delCmd := k.sendKeyPress(string(protocol.SpecialKeyDelete))
				if cmd != nil {
					return k, tea.Batch(cmd, delCmd)
				}
				return k, delCmd
			}
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
	}

	// For any other message type (including first message from Init), read from socket
	return k, k.ReadSocket
}

func (k *Keyboard) toggleVoiceMode() {
	if _, isVoice := k.model.(*VoiceModel); isVoice {
		// Switch back to prior model
		if k.priorModel != nil {
			k.model = k.priorModel
			k.priorModel = nil
			resizeKeyboardPane(k.paneHeight)
			slog.Info("voice mode off")
		}
	} else {
		// Switch to voice mode
		k.priorModel = k.model
		k.model = NewVoiceModel(defaultSTTProvider())
		resizeKeyboardPane(1)
		slog.Info("voice mode on")
	}
}

func resizeKeyboardPane(height int) {
	exec.Command("tmux", "resize-pane", "-t", "1", "-y", fmt.Sprintf("%d", height)).Run()
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
			k.toggleVoiceMode()
			return nil
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
		return vm.DrainCmd()
	}

	if keyToPress == nil {
		return nil
	}

	// Intercept MIC key to toggle voice mode
	if len(*keyToPress) == 1 && rune((*keyToPress)[0]) == protocol.SpecialKeyMic {
		k.toggleVoiceMode()
		return nil
	}

	// Track typed characters for autocomplete
	k.updateCurrentWord(*keyToPress)

	return k.sendKeyPress(*keyToPress)
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
