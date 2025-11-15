//go:build sdl
// +build sdl

package keyboard

import (
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
)

// no-op, as SDL input comes from the daemon
func (k Keyboard) handleKeyboardInput(msg tea.KeyMsg) tea.Cmd {
	slog.Warn("Got keyboard input in SDL mode", "msg", msg)
	return nil
}
