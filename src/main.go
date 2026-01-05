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
	fmt.Fprintf(os.Stderr, `usage: rex
       rex server
       rex keyboard [options]
       rex menu [options]
       rex exclusive <path>

Use 'rex <command> --help' for more information about a command.
`)
}

func main() {
	if len(os.Args) < 2 {
		if err := startOrAttach(); err != nil {
			slog.Error("error", "err", err)
			os.Exit(1)
		}
		return
	}

	subcommand := os.Args[1]
	args := os.Args[2:]

	var err error
	switch subcommand {
	case "-h", "-help", "--help":
		usage()
	case "server":
		fs := flag.NewFlagSet("server", flag.ExitOnError)
		fs.Usage = func() { fmt.Fprintf(os.Stderr, "usage: rex server\n\nStart the rex server.\n") }
		fs.Parse(args)
		err = server.NewServer().Run()
	case "keyboard":
		fs := flag.NewFlagSet("keyboard", flag.ExitOnError)
		fs.Usage = func() { fmt.Fprintf(os.Stderr, "usage: rex keyboard [options]\n\n"); fs.PrintDefaults() }
		keyboardMode := fs.String("mode", "chord", "keyboard mode (simple or chorded)")
		fs.Parse(args)
		err = keyboard.NewKeyboard(*keyboardMode).Run()
	case "menu":
		fs := flag.NewFlagSet("menu", flag.ExitOnError)
		fs.Usage = func() { fmt.Fprintf(os.Stderr, "usage: rex menu [options]\n\n"); fs.PrintDefaults() }
		configPath := fs.String("config", "", "path to config file (default: ~/.config/rex/config.yaml, $PWD/config.yaml)")
		sshHostFlag := fs.String("ssh", "", "SSH host to open app in")
		fs.Parse(args)

		var sshHost *string
		if *sshHostFlag != "" {
			sshHost = sshHostFlag
		}

		var m *menu.Menu
		m, err = menu.NewMenu(*configPath, sshHost)
		if err == nil {
			err = m.Run()
		}
	case "exclusive":
		fs := flag.NewFlagSet("exclusive", flag.ExitOnError)
		fs.Usage = func() {
			fmt.Fprintf(os.Stderr, "usage: rex exclusive <path>\n\nRun an application exclusively, stopping the rex server temporarily.\n")
		}
		fs.Parse(args)
		if fs.NArg() < 1 {
			fs.Usage()
			os.Exit(1)
		}
		err = runExclusive(fs.Arg(0))
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", subcommand)
		usage()
		os.Exit(1)
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
