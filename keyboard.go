//go:build !sdl
// +build !sdl

package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyUp:
			m.moveSelection("up")
		case tea.KeyDown:
			m.moveSelection("down")
		case tea.KeyLeft:
			m.moveSelection("left")
		case tea.KeyRight:
			m.moveSelection("right")
		case tea.KeyEnter:
			m.handleKeySelection()
		}
	}
	return m, nil
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
