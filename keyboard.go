//go:build !sdl
// +build !sdl

package main

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (k KeyboardView) Init() tea.Cmd {
	return nil
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
		case tea.KeyEnter:
			m.model.HandleKey(keyA)
		}
	}
	return m, nil
}