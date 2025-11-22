package server

import (
	"fmt"
	"log"
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

	log.Println("sending key", tmuxKey)
	// Send keys to pane 0 (top pane) - the main pane, not the keyboard pane
	cmd := exec.Command("tmux", "send-keys", "-t", "0", tmuxKey)
	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("tmux command failed: %v, output: %s", err, string(output))
		return err
	}

	return nil
}
