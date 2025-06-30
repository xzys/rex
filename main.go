package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xzys/TR3X/v2/keyboard"
)

// Build flags:
// -tags sdl for SDL joystick support
// -tags keyboard for keyboard support
// Default to keyboard if no tags specified

func main() {
	m := keyboard.NewKeyboardView()
	p := tea.NewProgram(m)
	defer m.Cleanup()

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
