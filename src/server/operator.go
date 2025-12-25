package server

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

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
func (o *GlobalOperator) HandleKey(key int) error {
	var err error
	if o.keyboardActive {
		switch key {
		case protocol.KeyR2:
			err = o.closeKeyboardPane()
		default:
			// send key input message to keyboard
			err = o.server.SendKeyInputMessage(key)
		}
	} else {
		// sync active operator with current window
		o.updateActiveOperator()
		// try active operator first
		if o.activeOperator != nil {
			handled, err := (*o.activeOperator).HandleKey(key)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
		}
		// fall back to global keys
		switch key {
		case protocol.KeyR1:
			err = o.nextWindow()
		case protocol.KeyL1:
			err = o.prevWindow()
		case protocol.KeyR2:
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

	slog.Info("opening keyboard pane")
	// Create the split pane
	if err := execute("tmux", "split-window",
		"-l", "9", // set pane size to 10 lines tall
		"-t", "0", // target pane 0
		binaryName, "keyboard", // command to run
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
	return execute("tmux", "next-window")
}

func (o *GlobalOperator) prevWindow() error {
	return execute("tmux", "previous-window")
}

func (o *GlobalOperator) newWindow() error {
	return execute("tmux", "new-window")
}
