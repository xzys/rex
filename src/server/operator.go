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

	var err error
	if o.keyboardActive {
		if event.Key == protocol.KeySelect && event.IsDown {
			return o.closeKeyboardPane()
		}
		// send key input message to keyboard
		return o.server.SendKeyInputMessage(event)
	} else {
		if !event.IsDown {
			return nil
		}
		// sync active operator with current window
		o.updateActiveOperator()
		// try active operator first
		if o.activeOperator != nil {
			handled, err := (*o.activeOperator).HandleKey(event.Key)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
		}
		// fall back to global keys
		switch event.Key {
		case protocol.KeyR1:
			err = o.nextWindow()
		case protocol.KeyL1:
			err = o.prevWindow()
		case protocol.KeySelect:
			err = o.openKeyboardPane()
		case protocol.KeyL2:
			err = o.newWindow()
		case protocol.KeyStart:
			err = o.openMenu()
		case protocol.KeyDPadUp:
			err = execute("tmux", "send-keys", "-t", "0", "Up")
		case protocol.KeyDPadDown:
			err = execute("tmux", "send-keys", "-t", "0", "Down")
		case protocol.KeyDPadLeft:
			err = execute("tmux", "send-keys", "-t", "0", "Left")
		case protocol.KeyDPadRight:
			err = execute("tmux", "send-keys", "-t", "0", "Right")
		}
	}
	return err
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

// creates a horizontal split at the bottom running the keyboard command
func (o *GlobalOperator) openKeyboardPane() error {
	// Check if process is already running
	if err := execute("pgrep", "-f", binaryName+" keyboard"); err == nil {
		slog.Info("keyboard pane already exists")
		return nil
	}

	mode := cfg.KeyboardModeChorded
	if loaded, err := cfg.LoadConfig(); err == nil {
		mode = cfg.NormalizeKeyboardMode(loaded.KeyboardMode)
	} else {
		slog.Warn("failed to load config, using default keyboard mode", "err", err)
	}

	slog.Info("opening keyboard pane", "mode", mode)
	// Create the split pane
	if err := execute("tmux", "split-window",
		"-l", "9", // set pane size to 10 lines tall
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

func (o *GlobalOperator) newWindow() error {
	return execute("tmux", "new-window")
}
