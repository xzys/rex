//go:build !sdl
// +build !sdl

package keyboard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/xzys/TR3X/v2/protocol"
)

func (k KeyboardView) HandleKeyMessage(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return tea.Quit
		case tea.KeyUp:
			k.model.HandleKey(protocol.KeyDPadUp)
		case tea.KeyDown:
			k.model.HandleKey(protocol.KeyDPadDown)
		case tea.KeyLeft:
			k.model.HandleKey(protocol.KeyDPadLeft)
		case tea.KeyRight:
			k.model.HandleKey(protocol.KeyDPadRight)
		case tea.KeyRunes:
			switch string(msg.Runes) {
			case "w":
				k.model.HandleKey(protocol.KeyY)
			case "a":
				k.model.HandleKey(protocol.KeyX)
			case "s":
				k.model.HandleKey(protocol.KeyB)
			case "d":
				k.model.HandleKey(protocol.KeyA)
			// simulate L1/R1/L2/R2
			case "z":
				k.model.HandleKey(protocol.KeyL1)
			case "x":
				k.model.HandleKey(protocol.KeyR1)
			case "c":
				k.model.HandleKey(protocol.KeyL2)
			case "v":
				k.model.HandleKey(protocol.KeyR2)
			}
		}
	}
	return nil
}
