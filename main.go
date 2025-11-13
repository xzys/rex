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

func runServer() {
	if err := server.Run(); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func runKeyboard(keyboardMode string) {
	m := keyboard.NewKeyboardView(keyboardMode)
	p := tea.NewProgram(m)
	defer m.Cleanup()

	if _, err := p.Run(); err != nil {
		slog.Error("keyboard error", "err", err)
		os.Exit(1)
	}
}
func main() {
	serverMode := flag.Bool("server", false, "run as server")
	keyboardMode := flag.String("keyboard", "chord", "keyboard mode (simple or chorded)")
	flag.Parse()

	// server handles events and executes actions
	slog.Info("starting", "server", *serverMode, "keyboard", *keyboardMode)
	if *serverMode {
		runServer()
	} else {
		runKeyboard(*keyboardMode)
	}
}
