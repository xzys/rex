package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	_colorPrimary   = lipgloss.Color("#d7dadc")
	_colorSecondary = lipgloss.Color("#626262")
	_colorYellow    = lipgloss.Color("#b59f3b")
	_colorGreen     = lipgloss.Color("#538d4e")
)

type keyState int

const (
	_keyStateUnselected keyState = iota
	_keyStateAbsent
	_keyStatePresent
	_keyStateCorrect
)

func (s keyState) color() lipgloss.Color {
	switch s {
	case _keyStateUnselected:
		return _colorPrimary
	case _keyStateAbsent:
		return _colorSecondary
	case _keyStatePresent:
		return _colorYellow
	case _keyStateCorrect:
		return _colorGreen
	default:
		return _colorPrimary
	}
}

type model struct {
	keyStates map[string]keyState
}

func initialModel() model {
	return model{
		keyStates: make(map[string]keyState),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyRunes:
			if len(msg.Runes) == 1 {
				key := string(msg.Runes[0])
				// Cycle through states when a key is pressed
				currentState := m.keyStates[key]
				m.keyStates[key] = (currentState + 1) % 4
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	topRow := m.viewKeyboardRow([]string{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"})
	midRow := m.viewKeyboardRow([]string{"A", "S", "D", "F", "G", "H", "J", "K", "L"})
	botRow := m.viewKeyboardRow([]string{"ENTER", "Z", "X", "C", "V", "B", "N", "M", "DELETE"})

	keys := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Padding(0, 2).Render(topRow),
		lipgloss.NewStyle().Padding(0, 4).Render(midRow),
		botRow,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(_keyStateUnselected.color()).
		Padding(0, 1).
		Render(keys)
}

func (m model) viewKeyboardRow(keys []string) string {
	keysRendered := make([]string, len(keys))
	for i, key := range keys {
		status := _keyStateUnselected
		if len(key) == 1 {
			if state, exists := m.keyStates[key]; exists {
				status = state
			}
		}
		keysRendered[i] = m.viewKey(key, status.color())
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom, keysRendered...)
}

func (m model) viewKey(key string, color lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder()).
		BorderForeground(color).
		Foreground(color).
		Render(key)
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
