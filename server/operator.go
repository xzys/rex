package server

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/xzys/TR3X/v2/protocol"
)

type GlobalOperator struct {
	keyboardActive bool
	server         *Server
	operators      map[string]Operator
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
			if err = o.closeKeyboardPane(); err == nil {
				o.SetKeyboardActive(false)
			}
		default:
			// send key input message to keyboard
			err = o.server.SendKeyInputMessage(key)
		}
	} else {
		switch key {
		case protocol.KeyR1:
			err = o.nextWindow()
		case protocol.KeyL1:
			err = o.prevWindow()
		case protocol.KeyR2:
			if err = o.openKeyboardPane(); err == nil {
				o.SetKeyboardActive(true)
			}
		case protocol.KeyL2:
		default:
			slog.Warn("key not mapped, ignoring", "key", key)
		}
	}
	return err
}

func (o *GlobalOperator) SetKeyboardActive(active bool) {
	o.keyboardActive = active
}

// executes a key press using tmux
func (o *GlobalOperator) PressKey(key string) error {
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

// returns the index of the currently focused pane in tmux
func (o *GlobalOperator) getActivePane() (string, error) {
	cmd := exec.Command("tmux", "display-message", "-p", "#{pane_index}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get active pane: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
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
	return nil
}

// kills the keyboard process, which will close the pane
func (o *GlobalOperator) closeKeyboardPane() error {
	return execute("pkill", "-f", binaryName+" keyboard")
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
