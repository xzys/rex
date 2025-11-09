package main

import (
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xzys/TR3X/v2/keyboard"
	_ "github.com/xzys/TR3X/v2/utils"
)

// Build flags:
// -tags sdl for SDL joystick support
// otherwise uses keyboard for local keyboard support

func main() {
	mode := "simple"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	m := keyboard.NewKeyboardView(mode)
	p := tea.NewProgram(m)
	defer m.Cleanup()

	if _, err := p.Run(); err != nil {
		slog.Error("error running program", "err", err)
		os.Exit(1)
	}
}
