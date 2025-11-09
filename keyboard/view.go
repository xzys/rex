package keyboard

import (
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
	keyboard [][]string
	width    int
	model    KeyboardModel
}

type KeyboardModel interface {
	// modify internal state based on key
	HandleKey(key int) error
	// takes key and position
	// returns foreground, background, key (same or override)
	RenderKey(i, j int, key string) (string, string, string)
	// cleanup resources
	Cleanup()
}

func NewKeyboardView(mode string) KeyboardView {
	var m KeyboardModel
	if mode == "chord" {
		m = NewKeyboardModelChorded(keyboardLayout)
	} else {
		m = NewKeyboardModelSimple()
	}
	return KeyboardView{
		keyboard: keyboardLayout,
		width:    0,
		model:    m,
	}
}

func (k KeyboardView) View() string {
	var rows []string
	for i, row := range k.keyboard {
		var keys []string
		for j, key := range row {
			fg, bg, key := k.model.RenderKey(i, j, key)
			style := lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color(fg)).
				Foreground(lipgloss.Color(bg))

			keys = append(keys, style.Render(key))
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Bottom, keys...)
		rowStr = lipgloss.NewStyle().Padding(0, keyboardOffsets[i]).Render(rowStr)
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
