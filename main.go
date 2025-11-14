package main

import (
	"flag"
	"fmt"
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

func runKeyboard(keyboardMode string) error {
	model := keyboard.GetKeyboardModel(keyboardMode)
	view := keyboard.NewKeyboardView(model)

	client, err := keyboard.NewClient(model)
	if err != nil {
		return fmt.Errorf("failed to create keyboard client: %w", err)
	}
	defer client.Close()

	// Start handling messages from server
	go func() {
		if err := client.HandleMessages(); err != nil {
			slog.Error("keyboard client error", "err", err)
			os.Exit(1)
		}
	}()

	// Run the UI
	p := tea.NewProgram(view)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func main() {
	serverMode := flag.Bool("server", false, "run as server")
	keyboardMode := flag.String("keyboard", "chord", "keyboard mode (simple or chorded)")
	flag.Parse()

	// server handles events and executes actions
	slog.Info("starting", "server", *serverMode, "keyboard", *keyboardMode)
	var err error
	if *serverMode {
		err = server.Run()
	} else {
		err = runKeyboard(*keyboardMode)
	}
	if err != nil {
		slog.Error("error", "err", err)
		os.Exit(1)
	}
}
