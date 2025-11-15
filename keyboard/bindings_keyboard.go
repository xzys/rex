//go:build !sdl
// +build !sdl

package keyboard

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/xzys/TR3X/v2/protocol"
)

func (k Keyboard) handleKeyboardInput(key tea.KeyMsg) tea.Cmd {
	keyInput := -1
	switch key.Type {
	case tea.KeyCtrlC:
		return tea.Quit

	// translate keyboard input to protocol.KeyInputMessage
	case tea.KeyUp:
		keyInput = protocol.KeyDPadUp
	case tea.KeyDown:
		keyInput = protocol.KeyDPadDown
	case tea.KeyLeft:
		keyInput = protocol.KeyDPadLeft
	case tea.KeyRight:
		keyInput = protocol.KeyDPadRight
	case tea.KeyRunes:
		switch string(key.Runes) {
		case "m":
			keyInput = protocol.KeyY
		case ",":
			keyInput = protocol.KeyX
		case ".":
			keyInput = protocol.KeyB
		case "/":
			keyInput = protocol.KeyA
		// simulate L1/R1/L2/R2
		case "z":
			keyInput = protocol.KeyL1
		case "x":
			keyInput = protocol.KeyR1
		case "c":
			keyInput = protocol.KeyL2
		case "v":
			keyInput = protocol.KeyR2
		}
	}
	if keyInput == -1 {
		return nil
	}
	// forward to main message handler
	return k.handleDeviceInput(protocol.KeyInputMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput},
		KeyInput:    keyInput,
	})
}
