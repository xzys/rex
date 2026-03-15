package keyboard

import (
	"strings"

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

func (k *Keyboard) View() string {
	var rows []string
	for i, row := range k.keyboard {
		var keys []string
		for j, key := range row {
			fg, bg, key := k.model.RenderKey(i, j, key)
			border := lipgloss.NormalBorder()
			showBottom := !k.compact
			style := lipgloss.NewStyle().
				Padding(0, 1).
				Border(border, true, true, showBottom, true).
				BorderForeground(lipgloss.Color(bg)).
				Foreground(lipgloss.Color(fg))

			keys = append(keys, style.Render(key))
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Bottom, keys...)
		rowStr = lipgloss.NewStyle().Padding(0, keyboardOffsets[i]).Render(rowStr)
		rows = append(rows, rowStr)
	}

	// Render suggestion bar for autocomplete
	var suggestionBar string
	if k.dictionary != nil {
		var words []string
		for _, s := range k.suggestions {
			if s != "" {
				words = append(words, s)
			}
		}
		text := strings.Join(words, " | ")
		suggestionBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color("7")).
			Render(text)
	}

	allRows := rows
	if k.dictionary != nil {
		allRows = append([]string{suggestionBar}, rows...)
	}

	keys := lipgloss.JoinVertical(lipgloss.Center, allRows...)
	return lipgloss.NewStyle().
		Width(k.width).
		Align(lipgloss.Center).
		Render(keys)
}
