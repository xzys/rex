package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/xzys/TR3X/v2/keyboard"
	"github.com/xzys/TR3X/v2/server"
	"github.com/xzys/TR3X/v2/start_menu"
	_ "github.com/xzys/TR3X/v2/utils"
)

// Build flags:
// -tags sdl for SDL joystick support
// otherwise uses keyboard for local keyboard support

func usage() {
	fmt.Fprintf(os.Stderr, "usage: tr3x server\n")
	fmt.Fprintf(os.Stderr, "       tr3x keyboard [--mode MODE]\n")
	fmt.Fprintf(os.Stderr, "       tr3x menu [--config PATH]\n")
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
		configPath := flag.String("config", "config.yaml", "path to config file")
		flag.Parse()
		var menu *start_menu.StartMenu
		menu, err = start_menu.NewStartMenu(*configPath)
		if err == nil {
			err = menu.Run()
		}
	default:
		usage()
	}

	if err != nil {
		slog.Error("error", "err", err)
		os.Exit(1)
	}
}
