package keyboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Build flags:
// -tags sdl for SDL joystick support
// -tags keyboard for keyboard support
// Default to keyboard if no tags specified

// Common keyboard layout: four rows, split into two four-column hands.
var keyboardLayout = [][]string{
	{"q", "w", "e", "r", "u", "i", "o", "p"},
	{"a", "s", "d", "f", "h", "j", "k", "l"},
	{"z", "x", "c", "v", "b", "n", "m", "MIC"},
	{"123", "SFT", "g", "t", "y", "ENT", "DEL", "SPC"},
}

const (
	keyboardHandCols    = 4
	keyboardMinKeyWidth = 5
)

// implement tea.Model interface View

func (k *Keyboard) View() string {
	if vr, ok := k.model.(VoiceRenderer); ok {
		return vr.RenderView(k.width)
	}

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
			if lipgloss.Width(key) < keyboardMinKeyWidth {
				style = style.Width(keyboardMinKeyWidth).Align(lipgloss.Center)
			}
			if j == keyboardHandCols-1 {
				style = style.MarginRight(2)
			}

			keys = append(keys, style.Render(key))
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Bottom, keys...)
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
