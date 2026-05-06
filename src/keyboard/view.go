package keyboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	cfg "github.com/xrfr/t-rex/v2/config"
)

// Build flags:
// -tags sdl for SDL joystick support
// -tags keyboard for keyboard support
// Default to keyboard if no tags specified

type keyboardLayoutDefinition struct {
	keys        [][]string
	handCols    int
	rowOffsets  []int
	minKeyWidth int
}

// Four rows, split into two four-column hands.
var keyboardLayout4x8 = keyboardLayoutDefinition{
	keys: [][]string{
		{"q", "w", "e", "r", "u", "i", "o", "p"},
		{"a", "s", "d", "f", "h", "j", "k", "l"},
		{"z", "x", "c", "v", "b", "n", "m", "MIC"},
		{"123", "SFT", "g", "t", "y", "ENT", "DEL", "SPC"},
	},
	handCols:    4,
	minKeyWidth: 5,
}

// Original three-row keyboard layout.
var keyboardLayout3Row = keyboardLayoutDefinition{
	keys: [][]string{
		{"123", "q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "MIC"},
		{"SFT", "a", "s", "d", "f", "g", "h", "j", "k", "l", "SPC"},
		{"DEL", "z", "x", "c", "v", "b", "n", "m", "ENT"},
	},
	rowOffsets: []int{0, 2, 7},
}

const (
	keyboardBaseHandGap = 2
	keyboardFanStep     = 2 // Grow center gap by two so each hand fans out one char per row.
)

// implement tea.Model interface View

func (k *Keyboard) View() string {
	if vr, ok := k.model.(VoiceRenderer); ok {
		return vr.RenderView(k.width)
	}

	var rows []string
	totalRows := len(k.keyboard)
	for i, row := range k.keyboard {
		var keys []string
		handGap := k.handGapForRow(i, totalRows)
		rowOffset := k.rowOffsetForRow(i)
		for j, key := range row {
			fg, bg, key := k.model.RenderKey(i, j, key)
			border := lipgloss.NormalBorder()
			showBottom := !k.compact
			style := lipgloss.NewStyle().
				Padding(0, 1).
				Border(border, true, true, showBottom, true).
				BorderForeground(lipgloss.Color(bg)).
				Foreground(lipgloss.Color(fg))
			if k.minKeyWidth > 0 && lipgloss.Width(key) < k.minKeyWidth {
				style = style.Width(k.minKeyWidth).Align(lipgloss.Center)
			}
			if k.handCols > 0 && j == k.handCols-1 {
				style = style.MarginRight(handGap)
			}

			keys = append(keys, style.Render(key))
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Bottom, keys...)
		if rowOffset > 0 {
			rowStr = lipgloss.NewStyle().Padding(0, rowOffset).Render(rowStr)
		}
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

func (k *Keyboard) handGapForRow(rowIndex, totalRows int) int {
	if k.keyboardView != cfg.KeyboardViewFan {
		return keyboardBaseHandGap
	}
	return keyboardBaseHandGap + (totalRows-1-rowIndex)*keyboardFanStep
}

func (k *Keyboard) rowOffsetForRow(rowIndex int) int {
	if rowIndex < 0 || rowIndex >= len(k.rowOffsets) {
		return 0
	}
	return k.rowOffsets[rowIndex]
}

func keyboardLayoutForName(name string) keyboardLayoutDefinition {
	if cfg.NormalizeKeyboardLayout(name) == cfg.KeyboardLayout3Row {
		return keyboardLayout3Row
	}
	return keyboardLayout4x8
}
