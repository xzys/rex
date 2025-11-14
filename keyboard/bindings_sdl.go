//go:build sdl
// +build sdl

package keyboard

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (k KeyboardView) HandleKeyMessage(msg tea.Msg) tea.Cmd {
	// handled in server
	return nil
}
