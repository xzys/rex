package keyboard

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Build flags:
// -tags sdl for SDL joystick support
// -tags keyboard for keyboard support
// Default to keyboard if no tags specified

// Common keyboard layout
var keyboardLayout = [][]string{
	{"123", "q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "BCK"},
	{"SFT", "a", "s", "d", "f", "g", "h", "j", "k", "l", "SPC"},
	{"DEL", "z", "x", "c", "v", "b", "n", "m", "ENT"},
}
var keyboardOffsets = []int{0, 2, 7}

// implement tea.Model interface View

func (k Keyboard) View() string {
	var rows []string
	for i, row := range k.keyboard {
		var keys []string
		for j, key := range row {
			fg, bg, key := k.model.RenderKey(i, j, key)
			style := lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color(bg)).
				Foreground(lipgloss.Color(fg))

			keys = append(keys, style.Render(key))
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Bottom, keys...)
		rowStr = lipgloss.NewStyle().Padding(0, keyboardOffsets[i]).Render(rowStr)
		rows = append(rows, rowStr)
	}

	// Render suggestion bar if the model supports autocomplete
	var suggestionBar string
	if fc, ok := k.model.(*KeyboardModelFrontChorded); ok {
		suggestions := fc.Suggestions()
		labels := [4]string{"L1", "L2", "R1", "R2"}
		var slots []string
		for i, s := range suggestions {
			if s != "" {
				slot := lipgloss.NewStyle().
					Padding(0, 1).
					Border(lipgloss.NormalBorder()).
					BorderForeground(lipgloss.Color("4")).
					Foreground(lipgloss.Color("7")).
					Render(fmt.Sprintf("[%s] %s", labels[i], s))
				slots = append(slots, slot)
			}
		}
		if len(slots) > 0 {
			suggestionBar = lipgloss.JoinHorizontal(lipgloss.Bottom, slots...)
		}
	}

	allRows := rows
	if suggestionBar != "" {
		allRows = append([]string{suggestionBar}, rows...)
	}

	keys := lipgloss.JoinVertical(lipgloss.Left, allRows...)
	return lipgloss.NewStyle().
		Width(k.width).
		Align(lipgloss.Center).
		Render(
			lipgloss.NewStyle().
				Padding(0, 1).
				Render(keys),
		)
}
