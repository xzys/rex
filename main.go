package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Build flags:
// -tags sdl for SDL joystick support
// -tags keyboard for keyboard support
// Default to keyboard if no tags specified

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
	keyStates   map[string]keyState
	selectedRow int
	selectedCol int
	keyboard    [][]string
	width       int
}

// Common keyboard layout
var keyboardLayout = [][]string{
	{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"},
	{"A", "S", "D", "F", "G", "H", "J", "K", "L"},
	{"ENTER", "Z", "X", "C", "V", "B", "N", "M", "DELETE"},
}

func initialModel() model {
	return model{
		keyStates:   make(map[string]keyState),
		selectedRow: 0,
		selectedCol: 0,
		keyboard:    keyboardLayout,
		width:       0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

// Common function to handle key selection
func (m *model) handleKeySelection() {
	key := m.keyboard[m.selectedRow][m.selectedCol]
	if len(key) == 1 {
		currentState := m.keyStates[key]
		m.keyStates[key] = (currentState + 1) % 4
		fmt.Printf("Selected key: %s, new state: %d\n", key, m.keyStates[key])
	} else {
		fmt.Printf("Selected special key: %s\n", key)
	}
}

// Common function to handle navigation
func (m *model) moveSelection(direction string) {
	switch direction {
	case "up":
		if m.selectedRow > 0 {
			m.selectedRow--
			if m.selectedCol >= len(m.keyboard[m.selectedRow]) {
				m.selectedCol = len(m.keyboard[m.selectedRow]) - 1
			}
		}
	case "down":
		if m.selectedRow < len(m.keyboard)-1 {
			m.selectedRow++
			if m.selectedCol >= len(m.keyboard[m.selectedRow]) {
				m.selectedCol = len(m.keyboard[m.selectedRow]) - 1
			}
		}
	case "left":
		if m.selectedCol > 0 {
			m.selectedCol--
		}
	case "right":
		if m.selectedCol < len(m.keyboard[m.selectedRow])-1 {
			m.selectedCol++
		}
	}
}

func (m model) View() string {
	var rows []string
	for i, row := range m.keyboard {
		var keys []string
		for j, key := range row {
			status := _keyStateUnselected
			if len(key) == 1 {
				if state, exists := m.keyStates[key]; exists {
					status = state
				}
			}
			style := lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.NormalBorder()).
				BorderForeground(status.color()).
				Foreground(status.color())

			// Highlight selected key
			if i == m.selectedRow && j == m.selectedCol {
				style = style.BorderForeground(lipgloss.Color("2")).Foreground(lipgloss.Color("2"))
			}

			keys = append(keys, style.Render(key))
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Bottom, keys...)
		if i == 0 {
			rowStr = lipgloss.NewStyle().Padding(0, 2).Render(rowStr)
		} else if i == 1 {
			rowStr = lipgloss.NewStyle().Padding(0, 4).Render(rowStr)
		}
		rows = append(rows, rowStr)
	}

	keys := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().
		Width(m.width).
		Align(lipgloss.Center).
		Render(
			lipgloss.NewStyle().
				Padding(0, 1).
				Render(keys),
		)
}
