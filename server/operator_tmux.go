package server

import (
	"fmt"
	"log/slog"
	"os/exec"

	"github.com/xzys/TR3X/v2/protocol"
)

// pressKey executes a key press using tmux
func (s *Server) pressKey(key string) error {
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

	slog.Info("sending key", "key", tmuxKey)
	// Send keys to pane 0 (top pane, main pane)
	cmd := exec.Command("tmux", "send-keys", "-t", "0", tmuxKey)
	if output, err := cmd.CombinedOutput(); err != nil {
		slog.Error("tmux command failed", "err", err, "output", string(output))
		return err
	}

	return nil
}

// openKeyboardPane creates a horizontal split at the bottom running the keyboard command
func (s *Server) openKeyboardPane() error {
	// Check if process is already running
	pgrepCmd := exec.Command("pgrep", "-f", binaryName+" keyboard")
	if err := pgrepCmd.Run(); err == nil {
		slog.Info("keyboard pane already exists")
		return nil
	}

	slog.Info("opening keyboard pane")
	// Create the split pane
	cmd := exec.Command("tmux", "split-window",
		"-l", "10", // set pane size to 10 lines tall
		"-t", "0", // target pane 0
		binaryName, "keyboard", // command to run
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		slog.Error("tmux split-window failed", "err", err, "output", string(output))
		return fmt.Errorf("failed to open keyboard pane: %w", err)
	}
	return nil
}

// closeKeyboardPane kills the keyboard process, which will close the pane
func (s *Server) closeKeyboardPane() error {
	slog.Info("closing keyboard pane")
	cmd := exec.Command("pkill", "-f", binaryName+" keyboard")
	if err := cmd.Run(); err != nil {
		slog.Warn("failed to close keyboard pane", "err", err) // not fatal
	}
	return nil
}
