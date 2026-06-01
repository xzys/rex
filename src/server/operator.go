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
	// is open.
	switch event.Key {
	case protocol.KeyL1:
		if event.IsDown {
			o.l1Down = true
			o.l1DownAt = time.Now()
			if o.r1Down {
				o.comboFired = true
				o.killKeyboardIfActive()
				if time.Since(o.r1DownAt) <= shoulderChordWindow {
					return execute("tmux", "kill-pane")
				}
				return execute("tmux", "select-pane", "-t", ":.-")
			}
			return nil
		}
		o.l1Down = false
		if !o.r1Down && !o.comboFired {
			o.killKeyboardIfActive()
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
				o.killKeyboardIfActive()
				if time.Since(o.l1DownAt) <= shoulderChordWindow {
					return execute("tmux", "kill-pane")
				}
				return execute("tmux", "select-pane", "-t", ":.+")
			}
			return nil
		}
		o.r1Down = false
		if !o.l1Down && !o.comboFired {
			o.killKeyboardIfActive()
			return o.nextWindow()
		}
		o.comboFired = false
		return nil
	}

	if o.keyboardActive {
		if event.Key == protocol.KeySelect && event.IsDown {
			return o.closeKeyboardPane()
		}
		// Keys the keyboard doesn't handle fall through to global
		switch event.Key {
		case protocol.KeyScrollUp, protocol.KeyScrollDown, protocol.KeyR3:
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
		return execute("tmux", "send-keys", "-t", "0", "Enter")
	case protocol.KeyB:
		return execute("tmux", "send-keys", "-t", "0", "Escape")
	case protocol.KeyScrollUp:
		return execute("tmux", "copy-mode", "-eu", "-t", "0")
	case protocol.KeyScrollDown:
		return tryExecuteIgnoreMessages([]string{"not in a mode"}, "tmux", "send-keys", "-t", "0", "-X", "page-down")
	case protocol.KeyR3:
		return execute("tmux", "send-keys", "-t", "0", "/")
	}
	return o.executeFallbackKey(key)
}

// executeFallbackKey handles the D-pad arrow keys in the global fallback.
// Extracted from executeFallback so the repeater can call it synthetically.
func (o *GlobalOperator) executeFallbackKey(key int) error {
	switch key {
	case protocol.KeyDPadUp:
		return execute("tmux", "send-keys", "-t", "0", "Up")
	case protocol.KeyDPadDown:
		return execute("tmux", "send-keys", "-t", "0", "Down")
	case protocol.KeyDPadLeft:
		return execute("tmux", "send-keys", "-t", "0", "Left")
	case protocol.KeyDPadRight:
		return execute("tmux", "send-keys", "-t", "0", "Right")
	}
	return nil
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
		return execute("tmux", "send-keys", "-t", "0", key)
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
	// Send keys to pane 0 (top pane, main pane)
	return execute("tmux", "send-keys", "-t", "0", tmuxKey)
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
// command. When forceVoice is true, the pane opens in voice mode (single-line
// height, --voice flag) regardless of the user's DefaultVoice config — used by
// the menu to auto-open voice for apps flagged `voice: true`.
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

	args := []string{"split-window"}
	if voice {
		args = append(args, "-l", "1") // voice mode uses a single line
	} else {
		args = append(args, "-l", fmt.Sprintf("%d", cfg.KeyboardPaneHeightForLayout(compact, autocomplete, layout)))
	}
	args = append(args, "-t", "0", binaryName, "keyboard", "--mode", mode)
	if voice {
		args = append(args, "--voice")
	}

	slog.Info("opening keyboard pane", "mode", mode, "layout", layout, "compact", compact, "autocomplete", autocomplete, "voice", voice)
	if err := execute("tmux", args...); err != nil {
		return fmt.Errorf("failed to open keyboard pane: %w", err)
	}
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

// Dismiss the keyboard on any pane/window nav — input is socket-routed to it
// while active, so leaving focus would strand the user typing into a dead pane.
func (o *GlobalOperator) killKeyboardIfActive() {
	if !o.keyboardActive {
		return
	}
	if err := o.closeKeyboardPane(); err != nil {
		slog.Warn("failed to close keyboard on nav", "err", err)
	}
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
	return tryExecuteIgnoreMessages([]string{"no next window"}, "tmux", "next-window")
}

func (o *GlobalOperator) prevWindow() error {
	return tryExecuteIgnoreMessages([]string{"no previous window"}, "tmux", "previous-window")
}
