package server

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	cfg "github.com/xrfr/t-rex/v2/config"
	"github.com/xrfr/t-rex/v2/protocol"
)

type GlobalOperator struct {
	keyboardActive bool
	server         *Server
	operators      map[string]Operator
	activeOperator *Operator
	l1Down         bool
	r1Down         bool
	comboFired     bool
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

	if o.keyboardActive {
		if event.Key == protocol.KeySelect && event.IsDown {
			return o.closeKeyboardPane()
		}
		// send key input message to keyboard
		return o.server.SendKeyInputMessage(event)
	}

	// key-up: L1/R1 pane navigation + active operator
	if !event.IsDown {
		switch event.Key {
		case protocol.KeyL1:
			o.l1Down = false
			if !o.r1Down && !o.comboFired {
				return execute("tmux", "select-pane", "-t", ":.-")
			}
			o.comboFired = false
			return nil
		case protocol.KeyR1:
			o.r1Down = false
			if !o.l1Down && !o.comboFired {
				return execute("tmux", "select-pane", "-t", ":.+")
			}
			o.comboFired = false
			return nil
		}
		if o.activeOperator != nil {
			_, err := (*o.activeOperator).HandleInput(event.Key, event.IsDown)
			return err
		}
		return nil
	}

	// key-down: L1/R1 window navigation, then active operator, then global fallback
	switch event.Key {
	case protocol.KeyL1:
		o.l1Down = true
		if o.r1Down {
			o.comboFired = true
			return o.prevWindow()
		}
		return nil
	case protocol.KeyR1:
		o.r1Down = true
		if o.l1Down {
			o.comboFired = true
			return o.nextWindow()
		}
		return nil
	}

	o.updateActiveOperator()
	if o.activeOperator != nil {
		handled, err := (*o.activeOperator).HandleInput(event.Key, event.IsDown)
		if err != nil || handled {
			return err
		}
	}

	switch event.Key {
	case protocol.KeySelect:
		return o.openKeyboardPane()
	case protocol.KeyStart:
		return o.openMenu()
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

func (o *GlobalOperator) SetKeyboardActive(active bool) {
	o.keyboardActive = active
}

// executes a key press using tmux
func (o *GlobalOperator) HandleKeyPress(key string) error {
	if len(key) == 0 {
		return nil
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

// creates a horizontal split at the bottom running the keyboard command
func (o *GlobalOperator) openKeyboardPane() error {
	// Check if process is already running
	if err := execute("pgrep", "-f", binaryName+" keyboard"); err == nil {
		slog.Info("keyboard pane already exists")
		return nil
	}

	mode := cfg.KeyboardModeChorded
	compact := false
	if loaded, err := cfg.LoadConfig(); err == nil {
		mode = cfg.NormalizeKeyboardMode(loaded.KeyboardMode)
		compact = loaded.KeyboardCompact
	} else {
		slog.Warn("failed to load config, using default keyboard mode", "err", err)
	}

	paneHeight := "12" // 3 rows × 3 lines + suggestion bar + padding
	if compact {
		paneHeight = "8" // 3 rows × 2 lines + suggestion bar + padding
	}

	slog.Info("opening keyboard pane", "mode", mode, "compact", compact)
	// Create the split pane
	if err := execute("tmux", "split-window",
		"-l", paneHeight,
		"-t", "0", // target pane 0
		binaryName, "keyboard", "--mode", mode, // command to run
	); err != nil {
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

// opens the menu in a new window
func (o *GlobalOperator) openMenu() error {
	// Check if menu is already running and kill it if it is
	if err := execute("pgrep", "-f", binaryName+" menu"); err == nil {
		slog.Info("menu already exists, killing it first")
		if err := execute("pkill", "-f", binaryName+" menu"); err != nil {
			return err
		}
	}

	slog.Info("opening menu")
	// Create a new window running the menu command
	return execute("tmux", "new-window", "-n", "menu", binaryName, "menu")
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
	return tryExecute("tmux", []string{"no next window"}, "next-window")
}

func (o *GlobalOperator) prevWindow() error {
	return tryExecute("tmux", []string{"no previous window"}, "previous-window")
}

