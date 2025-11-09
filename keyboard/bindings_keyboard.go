//go:build !sdl
// +build !sdl

package keyboard

import (
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
)

func (k KeyboardView) Init() tea.Cmd {
	return nil
}

func (m KeyboardView) Cleanup() {
	if m.model != nil {
		m.model.Cleanup()
	}
}

func (m KeyboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyUp:
			m.model.HandleKey(keyDPadUp)
		case tea.KeyDown:
			m.model.HandleKey(keyDPadDown)
		case tea.KeyLeft:
			m.model.HandleKey(keyDPadLeft)
		case tea.KeyRight:
			m.model.HandleKey(keyDPadRight)
		case tea.KeyRunes:
			slog.Debug("got rune", "rune", string(msg.Runes))
			switch string(msg.Runes) {
			case "w":
				m.model.HandleKey(keyY)
			case "a":
				m.model.HandleKey(keyX)
			case "s":
				m.model.HandleKey(keyB)
			case "d":
				m.model.HandleKey(keyA)
			// simulate L1/R1/L2/R2
			case "z":
				m.model.HandleKey(keyL1)
			case "x":
				m.model.HandleKey(keyR1)
			case "c":
				m.model.HandleKey(keyL2)
			case "v":
				m.model.HandleKey(keyR2)
			}
		}
	}
	return m, nil
}
