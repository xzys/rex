package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"syscall"

	"github.com/xrfr/t-rex/v2/keyboard"
	"github.com/xrfr/t-rex/v2/menu"
	"github.com/xrfr/t-rex/v2/server"
	_ "github.com/xrfr/t-rex/v2/utils"
)

// Build flags:
// -tags sdl for SDL joystick support
// otherwise uses keyboard for local keyboard support

func usage() {
	fmt.Fprintf(os.Stderr, "usage: rex\n")
	fmt.Fprintf(os.Stderr, "       rex server\n")
	fmt.Fprintf(os.Stderr, "       rex keyboard [--mode MODE]\n")
	fmt.Fprintf(os.Stderr, "       rex menu [--config PATH]\n")
	fmt.Fprintf(os.Stderr, "       rex exclusive <path>\n")
	os.Exit(1)
}

func main() {
	var subcommand string
	if len(os.Args) < 2 {
		subcommand = ""
	} else {
		subcommand = os.Args[1]
		os.Args = os.Args[1:]
	}
	flag.Parse()

	var err error
	switch subcommand {
	case "":
		err = startOrAttach()
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
	case "exclusive":
		if len(os.Args) < 2 {
			usage()
		}
		err = runExclusive(os.Args[1])
	default:
		usage()
	}

	if err != nil {
		slog.Error("error", "err", err)
		os.Exit(1)
	}
}

const tmuxSessionName = "rex"

func startOrAttach() error {
	if exec.Command("tmux", "has-session", "-t", tmuxSessionName).Run() == nil {
		cmd := exec.Command("tmux", "attach-session", "-t", tmuxSessionName)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	rexPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get rex path: %w", err)
	}

	exec.Command("pkill", "-f", rexPath+" server").Run()

	serverCmd := exec.Command(rexPath, "server")
	serverCmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := serverCmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	tmuxCmd := exec.Command("tmux", "new-session", "-s", tmuxSessionName)
	tmuxCmd.Stdout = os.Stdout
	tmuxCmd.Stderr = os.Stderr
	tmuxCmd.Stdin = os.Stdin
	if err := tmuxCmd.Start(); err != nil {
		serverCmd.Process.Kill()
		return fmt.Errorf("failed to start tmux: %w", err)
	}

	done := make(chan error, 2)
	go func() { done <- serverCmd.Wait() }()
	go func() { done <- tmuxCmd.Wait() }()

	<-done

	serverCmd.Process.Kill()
	tmuxCmd.Process.Signal(syscall.SIGHUP)
	serverCmd.Wait()
	tmuxCmd.Wait()

	return nil
}

func runExclusive(execPath string) error {
	rexPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get rex path: %w", err)
	}

	slog.Info("killing rex server")
	exec.Command("pkill", "-f", rexPath+" server").Run()

	slog.Info("starting exclusive app", "path", execPath)
	cmd := exec.Command(execPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		slog.Error("emulationstation exited with error", "err", err)
	}

	slog.Info("restarting rex server")
	cmd = exec.Command(rexPath, "server")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to restart server: %w", err)
	}

	return nil
}
