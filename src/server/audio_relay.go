package server

import (
	"fmt"
	"log/slog"
	"os/exec"
	"syscall"
)

// audioRelay streams mic audio from the local machine to a remote ALSA
// loopback sink via SSH. Callers run Start once, then Stop once.
//
// Pipeline:
//
//	arecord -f S16_LE -r 16000 -c 1 -t raw -q \
//	  | ssh [-i key] [-p port] user@host \
//	      aplay -q -f S16_LE -r 16000 -c 1 -D hw:Loopback,0,0
//
// The remote host must already have `snd-aloop` loaded and the user's
// `~/.asoundrc` routing `default` capture to `hw:Loopback,1,0`. See
// notes/plans/cross-device-mic-relay.md for bootstrap.
type audioRelay struct {
	arecord *exec.Cmd
	ssh     *exec.Cmd
}

func startAudioRelay(r RemotePane) (*audioRelay, error) {
	ar := exec.Command("arecord", "-f", "S16_LE", "-r", "16000", "-c", "1", "-t", "raw", "-q")

	sshArgs := []string{}
	if r.Key != "" {
		sshArgs = append(sshArgs, "-i", r.Key)
	}
	if r.Port != "" {
		sshArgs = append(sshArgs, "-p", r.Port)
	}
	// dropbear uses -y to skip hostkey prompt; OpenSSH doesn't understand it
	// but rex's target today is dropbear on the Brick. Keep a TODO to branch
	// per-client if we ever cross-build for non-dropbear hosts.
	sshArgs = append(sshArgs, "-y", r.Addr(),
		"aplay", "-q", "-f", "S16_LE", "-r", "16000", "-c", "1",
		"-D", "hw:Loopback,0,0",
	)
	sshCmd := exec.Command("ssh", sshArgs...)

	// arecord stdout → ssh stdin
	pipe, err := ar.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("pipe: %w", err)
	}
	sshCmd.Stdin = pipe

	// Put each child in its own process group so Stop can kill with SIGTERM
	// without taking out rex itself.
	ar.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	sshCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := sshCmd.Start(); err != nil {
		return nil, fmt.Errorf("ssh start: %w", err)
	}
	if err := ar.Start(); err != nil {
		_ = sshCmd.Process.Kill()
		return nil, fmt.Errorf("arecord start: %w", err)
	}

	slog.Info("claude: audio relay started", "remote", r.Addr())
	return &audioRelay{arecord: ar, ssh: sshCmd}, nil
}

func (a *audioRelay) Stop() {
	if a == nil {
		return
	}
	// Kill arecord first so ssh sees EOF and exits cleanly. Fall back to
	// killing ssh if it doesn't follow.
	if a.arecord != nil && a.arecord.Process != nil {
		_ = a.arecord.Process.Signal(syscall.SIGTERM)
	}
	if a.ssh != nil && a.ssh.Process != nil {
		_ = a.ssh.Process.Signal(syscall.SIGTERM)
	}
	// Reap.
	if a.arecord != nil {
		_ = a.arecord.Wait()
	}
	if a.ssh != nil {
		_ = a.ssh.Wait()
	}
	slog.Info("claude: audio relay stopped")
}
