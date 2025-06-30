package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Build flags:
// -tags sdl for SDL joystick support
// -tags keyboard for keyboard support
// Default to keyboard if no tags specified

// Common keyboard layout
var keyboardLayout = [][]string{
	{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"},
	{"A", "S", "D", "F", "G", "H", "J", "K", "L"},
	{"ENTER", "Z", "X", "C", "V", "B", "N", "M", "DELETE"},
}

const (
	keyDPadUp = iota
	keyDPadDown
	keyDPadLeft
	keyDPadRight
	keyA
	keyB
	keyX
	keyY
	keyL1
	keyR1
	keyL2
	keyR2
	keySelect
	keyStart
	keyFn
)

type KeyboardView struct {
	keyboard    [][]string
	width       int
	model       KeyboardModel
}

type KeyboardModel interface {
	HandleKey(key int) error
	RenderKey(i, j int) (string, string)
}

type KeyboardModelSimple struct {
	selectedRow int
	selectedCol int
	rowWidths   []int
}

func (m *KeyboardModelSimple) HandleKey(key int) error {
	switch key {
	case keyDPadUp:
		if m.selectedRow > 0 {
			m.selectedRow--
			if m.selectedCol >= m.rowWidths[m.selectedRow] {
				m.selectedCol = m.rowWidths[m.selectedRow] - 1
			}
		}
	case keyDPadDown:
		if m.selectedRow < len(m.rowWidths)-1 {
			m.selectedRow++
			if m.selectedCol >= m.rowWidths[m.selectedRow] {
				m.selectedCol = m.rowWidths[m.selectedRow] - 1
			}
		}
	case keyDPadLeft:
		if m.selectedCol > 0 {
			m.selectedCol--
		}
	case keyDPadRight:
		if m.selectedCol < m.rowWidths[m.selectedRow]-1 {
			m.selectedCol++
		}
	}
	return nil
}

func (m KeyboardModelSimple) RenderKey(i, j int) (string, string) {
	if i == m.selectedRow && j == m.selectedCol {
		return "2", "2"
	}
	return "0", "0"
}

func initialModel() KeyboardView {
	rowWidths := make([]int, len(keyboardLayout))
	for i, row := range keyboardLayout {
		rowWidths[i] = len(row)
	}
	model := KeyboardModelSimple{
		selectedRow: 0,
		selectedCol: 0,
		rowWidths:   rowWidths,
	}
	return KeyboardView{
		keyboard:    keyboardLayout,
		width:       0,
		model:       &model,
	}
}

func (k KeyboardView) View() string {
	var rows []string
	for i, row := range k.keyboard {
		var keys []string
		for j, key := range row {
			fg, bg := k.model.RenderKey(i, j)
			style := lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color(fg)).
				Foreground(lipgloss.Color(bg))

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
		Width(k.width).
		Align(lipgloss.Center).
		Render(
			lipgloss.NewStyle().
				Padding(0, 1).
				Render(keys),
		)
}


func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		log.Fatal("Error: %v", err)
	}
}