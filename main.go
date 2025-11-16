package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/xzys/TR3X/v2/keyboard"
	"github.com/xzys/TR3X/v2/server"
	_ "github.com/xzys/TR3X/v2/utils"
)

// Build flags:
// -tags sdl for SDL joystick support
// otherwise uses keyboard for local keyboard support

func main() {
	serverMode := flag.Bool("server", false, "run as server")
	keyboardMode := flag.String("keyboard", "chord", "keyboard mode (simple or chorded)")
	flag.Parse()

	// server handles events and executes actions
	slog.Info("starting", "server", *serverMode, "keyboard", *keyboardMode)
	var err error
	if *serverMode {
		err = server.NewServer().Run()
	} else {
		err = keyboard.NewKeyboard(*keyboardMode).Run()
	}
	if err != nil {
		slog.Error("error", "err", err)
		os.Exit(1)
	}
}
