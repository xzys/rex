package server

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	cfg "github.com/xrfr/t-rex/v2/config"
	"github.com/xrfr/t-rex/v2/keyrepeat"
	"github.com/xrfr/t-rex/v2/protocol"
)

const shoulderChordWindow = 100 * time.Millisecond

type GlobalOperator struct {
	keyboardActive bool
	keyboardPane   string // tmux pane id (%N) of the live keyboard pane, "" when none
	server         *Server
	operators      map[string]Operator
	activeOperator *Operator
	l1Down         bool
	r1Down         bool
	l1DownAt       time.Time
	r1DownAt       time.Time
	comboFired     bool
	navRepeater    *keyrepeat.KeyRepeater
}

// fallbackRepeatKeys lists the global-fallback keys that auto-repeat.
// D-pad directions only — B (Escape), A (Enter), and others don't repeat.
var fallbackRepeatKeys = []int{
	protocol.KeyDPadUp,
	protocol.KeyDPadDown,
	protocol.KeyDPadLeft,
	protocol.KeyDPadRight,
}

func NewGlobalOperator(server *Server, operators map[string]Operator) *GlobalOperator {
	return &GlobalOperator{
		keyboardActive: false,
		server:         server,
		operators:      operators,
	}
}

// keymappings for tmux/keyboard
func (o *GlobalOperator) HandleInput(event InputEvent) error {
	if o.server.IsDisabled() {
		return nil
	}

	// L1/R1 always drive pane/window navigation, regardless of whether a
	// keyboard pane is active. The front-chorded and voice models don't use
	// L1/R1 as modifiers, so hoisting this above the keyboardActive check
	// lets the user navigate panes while voice is recording or the keyboard
	// is open — the keyboard pane survives navigation and follows the user.
	switch event.Key {
	case protocol.KeyL1:
		if event.IsDown {
			o.l1Down = true
			o.l1DownAt = time.Now()
			if o.r1Down {
				o.comboFired = true
				if time.Since(o.r1DownAt) <= shoulderChordWindow {
					return o.killActivePane()
				}
				return o.cyclePane(-1)
			}
			return nil
		}
		o.l1Down = false
		// comboFired is cleared only on the release that leaves both shoulders
		// up — clearing on the first release would make the second release
		// look like a plain tap and fire a spurious window switch.
		if o.r1Down {
			return nil
		}
		if !o.comboFired {
			return o.prevWindow()
		}
		o.comboFired = false
		return nil
	case protocol.KeyR1:
		if event.IsDown {
			o.r1Down = true
			o.r1DownAt = time.Now()
			if o.l1Down {
				o.comboFired = true
				if time.Since(o.l1DownAt) <= shoulderChordWindow {
					return o.killActivePane()
				}
				return o.cyclePane(1)
			}
			return nil
		}
		o.r1Down = false
		if o.l1Down {
			return nil
		}
		if !o.comboFired {
			return o.nextWindow()
		}
		o.comboFired = false
		return nil
	}

	if o.keyboardActive {
		if event.Key == protocol.KeySelect && event.IsDown {
			return o.closeKeyboardPane()
		}
		// Keys the keyboard doesn't handle fall through to global.
		// L2/R2 need to reach the SSHTmuxOperator for remote tmux
		// pane/window control even when the keyboard is open.
		switch event.Key {
		case protocol.KeyScrollUp, protocol.KeyScrollDown, protocol.KeyR3,
			protocol.KeyL2, protocol.KeyR2, protocol.KeyL3:
			// fall through to global handler below
		default:
			return o.server.SendKeyInputMessage(event)
		}
	}

	// key-up: stop any active repeater, then active operator
	if !event.IsDown {
		o.stopNavRepeat(event.Key)
		if o.activeOperator != nil {
			_, err := (*o.activeOperator).HandleInput(event.Key, event.IsDown)
			return err
		}
		return nil
	}

	// key-down: active operator, then global fallback
	o.updateActiveOperator()
	if o.activeOperator != nil {
		handled, err := (*o.activeOperator).HandleInput(event.Key, event.IsDown)
		if err != nil || handled {
			o.startOperatorRepeat(event.Key, *o.activeOperator)
			return err
		}
	}

	err := o.executeFallback(event.Key)
	if err == nil {
		o.startFallbackRepeat(event.Key)
	}
	return err
}

func (o *GlobalOperator) SetKeyboardActive(active bool) {
	o.keyboardActive = active
	if !active {
		o.keyboardPane = ""
	}
}

// --- key-repeat helpers ---

func (o *GlobalOperator) stopNavRepeat(key int) {
	if o.navRepeater != nil {
		o.navRepeater.Stop(key)
	}
}

func (o *GlobalOperator) stopNavRepeatAll() {
	if o.navRepeater != nil {
		o.navRepeater.StopAll()
	}
}

// startOperatorRepeat starts auto-repeat for a key that was handled by an
// operator that lists it in RepeatKeys(). The operator is captured in the
// closure so the repeater keeps targeting the same operator even if the user
// switches windows mid-hold.
func (o *GlobalOperator) startOperatorRepeat(key int, op Operator) {
	for _, k := range op.RepeatKeys() {
		if k != key {
			continue
		}
		o.stopNavRepeatAll()
		o.navRepeater = keyrepeat.NewKeyRepeater(
			func(k int) error {
				_, err := op.HandleInput(k, true)
				return err
			},
			keyrepeat.Delay,
			keyrepeat.Interval,
		)
		o.navRepeater.Start(key)
		return
	}
}

// startFallbackRepeat starts auto-repeat for a key handled by the global
// fallback that appears in fallbackRepeatKeys.
func (o *GlobalOperator) startFallbackRepeat(key int) {
	for _, k := range fallbackRepeatKeys {
		if k != key {
			continue
		}
		o.stopNavRepeatAll()
		o.navRepeater = keyrepeat.NewKeyRepeater(
			func(k int) error {
				return o.executeFallbackKey(k)
			},
			keyrepeat.Delay,
			keyrepeat.Interval,
		)
		o.navRepeater.Start(key)
		return
	}
}

// executeFallback is the global fallback — keys not handled by any operator.
func (o *GlobalOperator) executeFallback(key int) error {
	switch key {
	case protocol.KeySelect:
		return o.OpenKeyboardPane(false)
	case protocol.KeyStart:
		return o.openMenu()
	case protocol.KeyA:
		return execute("tmux", "send-keys", "Enter")
	case protocol.KeyB:
		return execute("tmux", "send-keys", "Escape")
	case protocol.KeyScrollUp:
		return execute("tmux", "copy-mode", "-eu")
	case protocol.KeyScrollDown:
		return tryExecuteIgnoreMessages([]string{"not in a mode"}, "tmux", "send-keys", "-X", "page-down")
	case protocol.KeyR3:
		return execute("tmux", "send-keys", "/")
	case protocol.KeyL3:
		return o.toggleCopyMode()
	}
	return o.executeFallbackKey(key)
}

// executeFallbackKey handles the D-pad arrow keys in the global fallback.
// Extracted from executeFallback so the repeater can call it synthetically.
func (o *GlobalOperator) executeFallbackKey(key int) error {
	switch key {
	case protocol.KeyDPadUp:
		return o.sendDirectionalKey("Up", "scroll-up")
	case protocol.KeyDPadDown:
		return o.sendDirectionalKey("Down", "scroll-down")
	case protocol.KeyDPadLeft:
		return execute("tmux", "send-keys", "Left")
	case protocol.KeyDPadRight:
		return execute("tmux", "send-keys", "Right")
	}
	return nil
}

// sendDirectionalKey sends the given key to the active pane. When tmux is in
// copy-mode it sends the scroll-X command instead so the viewport scrolls
// rather than moving the cursor.
func (o *GlobalOperator) sendDirectionalKey(key, scrollCmd string) error {
	inMode, err := tmuxOutput("display-message", "-p", "#{pane_in_mode}")
	if err != nil {
		return err
	}
	if inMode == "1" {
		return tryExecuteIgnoreMessages(
			[]string{"not in a mode"},
			"tmux", "send-keys", "-X", scrollCmd,
		)
	}
	return execute("tmux", "send-keys", key)
}

// toggleCopyMode enters or exits tmux copy-mode: if the pane is already in a
// mode, send cancel; otherwise enter copy-mode with -eu (exit on scroll-up).
func (o *GlobalOperator) toggleCopyMode() error {
	inMode, err := tmuxOutput("display-message", "-p", "#{pane_in_mode}")
	if err != nil {
		return err
	}
	if inMode == "1" {
		return execute("tmux", "send-keys", "-X", "cancel")
	}
	return execute("tmux", "copy-mode", "-eu")
}

// executes a key press using tmux
func (o *GlobalOperator) HandleKeyPress(key string) error {
	if len(key) == 0 {
		return nil
	}

	// Multi-character inputs are tmux key names (e.g. "BSpace", "Space",
	// "Enter", "Up") that callers have already translated — pass through
	// verbatim. The single-character branch below handles in-band special
	// codes from the keyboard protocol.
	if len(key) > 1 {
		return execute("tmux", "send-keys", key)
	}

	char := rune(key[0])
	var tmuxKey string
	switch char {
	case protocol.SpecialKeySpace:
		tmuxKey = "Space"
	case protocol.SpecialKeyDelete:
		tmuxKey = "BSpace"
	case protocol.SpecialKeyEnter:
		tmuxKey = "Enter"
	// TODO following are handled entirely on the keyboard side, maybe remove?
	case protocol.SpecialKeyClose:
	case protocol.SpecialKeyShift:
	case protocol.SpecialKeyNumbers:
		return fmt.Errorf("special key %s not expected", string(char))
	default:
		tmuxKey = string(char)
	}
	// Send to the focused pane — the keyboard pane is created unfocused, so
	// this is always the pane the user is looking at.
	return execute("tmux", "send-keys", tmuxKey)
}

// executes a string as a sequence of key presses (e.g. autocomplete)
func (o *GlobalOperator) HandlePressString(value string) error {
	for _, ch := range value {
		if err := o.HandleKeyPress(string(ch)); err != nil {
			return err
		}
	}
	return nil
}

// OpenKeyboardPane creates a horizontal split at the bottom running the keyboard
// command. The split is created unfocused (-d): the keyboard reads input from
// the socket, so focus stays on the app pane and send-keys lands there. When
// forceVoice is true, the pane opens in voice mode (single-line height, --voice
// flag) regardless of the user's DefaultVoice config — used by the menu to
// auto-open voice for apps flagged `voice: true`.
func (o *GlobalOperator) OpenKeyboardPane(forceVoice bool) error {
	// Check if process is already running
	if err := execute("pgrep", "-f", binaryName+" keyboard"); err == nil {
		slog.Info("keyboard pane already exists")
		return nil
	}

	mode := cfg.KeyboardModeFrontChorded
	layout := cfg.KeyboardLayout4x8
	compact := true
	autocomplete := false
	voice := forceVoice
	if loaded, err := cfg.LoadConfig(); err == nil {
		mode = loaded.KeyboardMode
		layout = cfg.NormalizeKeyboardLayout(loaded.KeyboardLayout)
		compact = loaded.KeyboardCompact
		autocomplete = loaded.KeyboardAutocomplete
		if loaded.DefaultVoice {
			voice = true
		}
	} else {
		slog.Warn("failed to load config, using default keyboard mode", "err", err)
	}

	args := []string{"split-window", "-d", "-P", "-F", "#{pane_id}"}
	if voice {
		args = append(args, "-l", "1") // voice mode uses a single line
	} else {
		args = append(args, "-l", fmt.Sprintf("%d", cfg.KeyboardPaneHeightForLayout(compact, autocomplete, layout)))
	}
	args = append(args, binaryName, "keyboard", "--mode", mode)
	if voice {
		args = append(args, "--voice")
	}

	slog.Info("opening keyboard pane", "mode", mode, "layout", layout, "compact", compact, "autocomplete", autocomplete, "voice", voice)
	paneID, err := tmuxOutput(args...)
	if err != nil {
		return fmt.Errorf("failed to open keyboard pane: %w", err)
	}
	o.keyboardPane = paneID
	o.SetKeyboardActive(true)
	return nil
}

// kills the keyboard process, which will close the pane
func (o *GlobalOperator) closeKeyboardPane() error {
	if err := execute("pkill", "-f", binaryName+" keyboard"); err != nil {
		return err
	}
	o.SetKeyboardActive(false)
	return nil
}

// cyclePane moves focus to the next (+1) or previous (-1) pane in the current
// window, skipping the keyboard pane — it never holds focus (input reaches it
// over the socket), so landing on it would strand send-keys output there.
func (o *GlobalOperator) cyclePane(step int) error {
	out, err := tmuxOutput("list-panes", "-F", "#{pane_id} #{pane_active}")
	if err != nil {
		return err
	}
	var ids []string
	active := -1
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] == o.keyboardPane {
			continue
		}
		if fields[1] == "1" {
			active = len(ids)
		}
		ids = append(ids, fields[0])
	}
	if active < 0 || len(ids) < 2 {
		return nil
	}
	next := ids[((active+step)%len(ids)+len(ids))%len(ids)]
	return execute("tmux", "select-pane", "-t", next)
}

// killActivePane kills the focused pane. The keyboard pane never holds focus,
// so this only ever kills app panes — but tmux may hand focus to the keyboard
// afterwards, which ensureFocusOffKeyboard corrects.
func (o *GlobalOperator) killActivePane() error {
	if err := execute("tmux", "kill-pane"); err != nil {
		return err
	}
	return o.ensureFocusOffKeyboard()
}

// ensureFocusOffKeyboard moves focus to an app pane when tmux has landed it on
// the keyboard pane (e.g. after kill-pane). If the keyboard is the only pane
// left in the window, close it so the window dies with it.
func (o *GlobalOperator) ensureFocusOffKeyboard() error {
	if o.keyboardPane == "" {
		return nil
	}
	active, err := tmuxOutput("display-message", "-p", "#{pane_id}")
	if err != nil || active != o.keyboardPane {
		return err
	}
	out, err := tmuxOutput("list-panes", "-F", "#{pane_id}")
	if err != nil {
		return err
	}
	for _, id := range strings.Fields(out) {
		if id != o.keyboardPane {
			return execute("tmux", "select-pane", "-t", id)
		}
	}
	return o.closeKeyboardPane()
}

// moveKeyboardToCurrentWindow re-attaches the live keyboard pane below the
// focused pane after a window switch. The keyboard process never restarts, so
// voice mode, the STT connection, and autocomplete state survive navigation.
func (o *GlobalOperator) moveKeyboardToCurrentWindow() error {
	if o.keyboardPane == "" {
		return nil
	}
	kbWindow, err := tmuxOutput("display-message", "-p", "-t", o.keyboardPane, "#{window_id}")
	if err != nil {
		// pane is gone — the keyboard died without telling us
		o.keyboardPane = ""
		return nil
	}
	curWindow, err := tmuxOutput("display-message", "-p", "#{window_id}")
	if err != nil {
		return err
	}
	if kbWindow == curWindow {
		return nil
	}
	height, err := tmuxOutput("display-message", "-p", "-t", o.keyboardPane, "#{pane_height}")
	if err != nil {
		return err
	}
	// -d keeps focus on the app pane; the keyboard only needs to be visible.
	return execute("tmux", "join-pane", "-d", "-v", "-l", height, "-s", o.keyboardPane)
}

// opens the menu in a new window
func (o *GlobalOperator) openMenu() error {
	// Anchor with $ so we only match the bare `rex menu` process, not apps
	// launched from the menu like `rex menu --ssh` (which would otherwise
	// get pkilled here and could take down the last tmux window with them).
	menuPattern := binaryName + " menu$"
	if err := execute("pgrep", "-f", menuPattern); err == nil {
		slog.Info("menu already exists, killing it first")
		if err := execute("pkill", "-f", menuPattern); err != nil {
			return err
		}
	}

	slog.Info("opening menu")
	// -c inherits the active pane's cwd so apps launched from the menu start
	// where the user was (e.g. opening claude from ~/notes lands in ~/notes,
	// not /home/rex). tmux expands #{pane_current_path} at command time.
	return execute("tmux", "new-window", "-c", "#{pane_current_path}", "-n", "menu", binaryName, "menu")
}

// set active operator based on the currently focused window name
func (o *GlobalOperator) updateActiveOperator() error {
	cmd := exec.Command("tmux", "display-message", "-p", "#{window_name}")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get window name: %w", err)
	}
	windowName := strings.TrimSpace(string(output))
	prev := o.activeOperator
	if operator, ok := o.operators[windowName]; ok {
		o.activeOperator = &operator
	} else {
		o.activeOperator = nil
	}
	if o.activeOperator != prev {
		slog.Debug("active operator changed", "window", windowName)
	}
	return nil
}

func (o *GlobalOperator) nextWindow() error {
	if err := tryExecuteIgnoreMessages([]string{"no next window"}, "tmux", "next-window"); err != nil {
		return err
	}
	return o.moveKeyboardToCurrentWindow()
}

func (o *GlobalOperator) prevWindow() error {
	if err := tryExecuteIgnoreMessages([]string{"no previous window"}, "tmux", "previous-window"); err != nil {
		return err
	}
	return o.moveKeyboardToCurrentWindow()
}
