package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/xrfr/t-rex/v2/keyboard"
	"github.com/xrfr/t-rex/v2/menu"
	"github.com/xrfr/t-rex/v2/server"
	_ "github.com/xrfr/t-rex/v2/utils"
)

// Build flags:
// -tags sdl for SDL joystick support
// otherwise uses keyboard for local keyboard support

func usage() {
	fmt.Fprintf(os.Stderr, "usage: rex server\n")
	fmt.Fprintf(os.Stderr, "       rex keyboard [--mode MODE]\n")
	fmt.Fprintf(os.Stderr, "       rex menu [--config PATH]\n")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	subcommand := os.Args[1]
	os.Args = os.Args[1:]
	flag.Parse()

	var err error
	switch subcommand {
	case "server":
		err = server.NewServer().Run()
	case "keyboard":
		keyboardMode := flag.String("mode", "chord", "keyboard mode (simple or chorded)")
		err = keyboard.NewKeyboard(*keyboardMode).Run()
	case "menu":
		configPath := flag.String("config", "", "path to config file (default: ~/.config/rex/config.yaml, $PWD/config.yaml)")
		flag.Parse()
		var m *menu.Menu
		m, err = menu.NewMenu(*configPath)
		if err == nil {
			err = m.Run()
		}
	default:
		usage()
	}

	if err != nil {
		slog.Error("error", "err", err)
		os.Exit(1)
	}
}
