package keyboard

import (
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
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

type KeyboardView struct {
	keyboard [][]string
	width    int
	model    KeyboardModel
}

func GetKeyboardModel(mode string) KeyboardModel {
	switch mode {
	case "chord":
		return NewKeyboardModelChorded(keyboardLayout)
	case "simple":
		return NewKeyboardModelSimple()
	default:
		slog.Error("invalid keyboard mode", "mode", mode)
		os.Exit(1)
		return nil
	}
}

func NewKeyboardView(model KeyboardModel) KeyboardView {
	return KeyboardView{
		keyboard: keyboardLayout,
		width:    0,
		model:    model,
	}
}

func (k KeyboardView) Init() tea.Cmd {
	return nil
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

func (m KeyboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		return m, m.HandleKeyMessage(msg)
	}
	return m, nil
}
