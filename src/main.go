package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	cfg "github.com/xrfr/t-rex/v2/config"
	"github.com/xrfr/t-rex/v2/keyboard"
	"github.com/xrfr/t-rex/v2/menu"
	"github.com/xrfr/t-rex/v2/server"
	"github.com/xrfr/t-rex/v2/settings"
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
       rex settings
       rex exclusive <command> [args...]

Use 'rex <command> --help' for more information about a command.
`)
}

func main() {
	// Ensure tools deployed alongside rex (tmux, etc.) are in PATH
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		os.Setenv("PATH", execDir+":"+os.Getenv("PATH"))
	}

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
		keyboardMode := fs.String("mode", "chorded", "keyboard mode (simple, chorded, chorded-simultaneous, front-chorded)")
		fs.Parse(args)

		config, _ := cfg.LoadConfig()
		mode := config.KeyboardMode
		if *keyboardMode != "" {
			mode = *keyboardMode
		}
		if mode == "" {
			mode = cfg.KeyboardModeChorded
		}
		dw := config.DictWords
		if dw <= 0 {
			dw = cfg.DefaultDictWords
		}
		sttMode := cfg.NormalizeSTTMode(config.STTMode)
		err = keyboard.NewKeyboard(mode, dw, config.KeyboardCompact, config.KeyboardAutocomplete, sttMode).Run()
	case "menu":
		fs := flag.NewFlagSet("menu", flag.ExitOnError)
		fs.Usage = func() { fmt.Fprintf(os.Stderr, "usage: rex menu [options]\n\n"); fs.PrintDefaults() }
		sshMode := fs.Bool("ssh", false, "select SSH host to open app in")
		fs.Parse(args)

		config, configErr := cfg.LoadConfig()
		if configErr != nil {
			err = configErr
			break
		}

		var sshHost *string
		if *sshMode {
			hosts, hostErr := menu.GetSSHHosts(config.SSHHosts)
			if hostErr != nil {
				err = fmt.Errorf("failed to get SSH hosts: %w", hostErr)
				break
			}
			selector := menu.NewSSHSelect(hosts)
			selected, selectErr := selector.Run()
			if selectErr != nil {
				err = selectErr
				break
			}
			if selected == "" {
				break
			}
			sshHost = &selected
		}

		m := menu.NewMenuWithConfig(config, sshHost)
		err = m.Run()
	case "settings":
		fs := flag.NewFlagSet("settings", flag.ExitOnError)
		fs.Usage = func() { fmt.Fprintf(os.Stderr, "usage: rex settings\n\nOpen settings UI.\n") }
		fs.Parse(args)
		err = settings.Run()
	case "exclusive":
		fs := flag.NewFlagSet("exclusive", flag.ExitOnError)
		fs.Usage = func() {
			fmt.Fprintf(os.Stderr, "usage: rex exclusive <command> [args...]\n\nRun an application exclusively, disabling rex input handling and switching to tty1.\n")
		}
		fs.Parse(args)
		if fs.NArg() < 1 {
			fs.Usage()
			os.Exit(1)
		}
		err = runExclusive(fs.Args())
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

func runExclusive(args []string) error {
	if err := sendDisableMessage(true); err != nil {
		slog.Warn("failed to send disable message", "err", err)
	}

	if err := exec.Command("chvt", "1").Run(); err != nil {
		slog.Warn("failed to switch to tty1", "err", err)
	}

	slog.Info("starting exclusive app", "args", args)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		slog.Error("exclusive app exited with error", "err", err)
	}

	if err := exec.Command("chvt", "2").Run(); err != nil {
		slog.Warn("failed to switch to tty2", "err", err)
	}

	if err := sendDisableMessage(false); err != nil {
		slog.Warn("failed to send enable message", "err", err)
	}

	return nil
}

func sendDisableMessage(disable bool) error {
	conn, err := net.Dial("unix", server.ControlSocketPath())
	if err != nil {
		return fmt.Errorf("failed to connect to control socket: %w", err)
	}
	defer conn.Close()

	msg := map[string]interface{}{
		"type":    "disable",
		"disable": disable,
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to send disable message: %w", err)
	}

	return nil
}
