package main

import (
	"flag"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xzys/TR3X/v2/keyboard"
	"github.com/xzys/TR3X/v2/server"
	_ "github.com/xzys/TR3X/v2/utils"
)

// Build flags:
// -tags sdl for SDL joystick support
// otherwise uses keyboard for local keyboard support

func main() {
	serverMode := flag.Bool("server", false, "run as server")
	flag.Parse()

	if *serverMode {
		// Run server mode
		if err := server.Run(); err != nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
		return
	}

	// Run client mode (keyboard UI)
	mode := "simple"
	if len(flag.Args()) > 0 {
		mode = flag.Args()[0]
	}
	m := keyboard.NewKeyboardView(mode)
	p := tea.NewProgram(m)
	defer m.Cleanup()

	if _, err := p.Run(); err != nil {
		slog.Error("error running program", "err", err)
		os.Exit(1)
	}
}
