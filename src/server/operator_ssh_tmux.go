package server

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/xrfr/t-rex/v2/protocol"
)

const sshInfoFile = "/tmp/rex-ssh-info"

// L2+R2 pressed within this window toggles zoom; longer holds fall back to
// previous/next-window so the existing window-switch gesture still works.
const sshTmuxComboWindow = 100 * time.Millisecond

type SSHTmuxOperator struct {
	l2Down     bool
	r2Down     bool
	l2DownAt   time.Time
	r2DownAt   time.Time
	comboFired bool // prevents the trailing key-up from firing a pane action after a window combo
}

func NewSSHTmuxOperator() *SSHTmuxOperator {
	return &SSHTmuxOperator{}
}

func (o *SSHTmuxOperator) RepeatKeys() []int {
	return []int{protocol.KeyDPadUp, protocol.KeyDPadDown}
}

func (o *SSHTmuxOperator) HandleInput(key int, isDown bool) (bool, error) {
	switch key {
	case protocol.KeyL2:
		o.l2Down = isDown
		if isDown {
			o.l2DownAt = time.Now()
			if o.r2Down {
				o.comboFired = true
				if time.Since(o.r2DownAt) <= sshTmuxComboWindow {
					return true, o.remoteCommand("resize-pane", "-Z")
				}
				return true, o.remoteCommand("previous-window")
			}
		} else if !o.r2Down {
			if !o.comboFired {
				return true, o.remoteCommand("select-pane", "-t", ":.-")
			}
			o.comboFired = false
		}
		return true, nil

	case protocol.KeyR2:
		o.r2Down = isDown
		if isDown {
			o.r2DownAt = time.Now()
			if o.l2Down {
				o.comboFired = true
				if time.Since(o.l2DownAt) <= sshTmuxComboWindow {
					return true, o.remoteCommand("resize-pane", "-Z")
				}
				return true, o.remoteCommand("next-window")
			}
		} else if !o.l2Down {
			if !o.comboFired {
				return true, o.remoteCommand("select-pane", "-t", ":.+")
			}
			o.comboFired = false
		}
		return true, nil

	case protocol.KeyScrollUp:
		if isDown {
			return true, o.remoteCommand("copy-mode", "-eu")
		}
		return true, nil
	case protocol.KeyScrollDown:
		if isDown {
			socket, host, err := readSSHInfo()
			if err != nil {
				return true, err
			}
			cmdArgs := []string{"-S", socket, "-o", "BatchMode=yes", host, "tmux", "send-keys", "-X", "page-down"}
			return true, tryExecuteIgnoreMessages([]string{"not in a mode"}, "ssh", cmdArgs...)
		}
		return true, nil

	case protocol.KeyDPadUp:
		if isDown {
			return true, o.remoteDirectionalKey("Up", "scroll-up")
		}
		return true, nil
	case protocol.KeyDPadDown:
		if isDown {
			return true, o.remoteDirectionalKey("Down", "scroll-down")
		}
		return true, nil

	case protocol.KeyL3:
		if isDown {
			return true, o.toggleCopyMode()
		}
		return true, nil
	}
	return false, nil
}

// remoteCommand runs a tmux command on the remote host via the ControlMaster socket.
func (o *SSHTmuxOperator) remoteCommand(args ...string) error {
	socket, host, err := readSSHInfo()
	if err != nil {
		return err
	}
	cmdArgs := append([]string{"-S", socket, "-o", "BatchMode=yes", host, "tmux"}, args...)

	// Mirror local behavior: trying to move past the edge window should not be fatal.
	if len(args) > 0 {
		switch args[0] {
		case "previous-window":
			return tryExecuteIgnoreMessages([]string{"no previous window"}, "ssh", cmdArgs...)
		case "next-window":
			return tryExecuteIgnoreMessages([]string{"no next window"}, "ssh", cmdArgs...)
		}
	}

	return execute("ssh", cmdArgs...)
}

// remoteDirectionalKey sends an arrow key to the remote pane, or — when the
// remote tmux is in copy-mode — a scroll-X command so the viewport scrolls
// (trackpad-style) instead of moving the cursor one line at a time.
func (o *SSHTmuxOperator) remoteDirectionalKey(key, scrollCmd string) error {
	inMode, err := o.remoteOutput("display-message", "-p", "\\#{pane_in_mode}")
	if err != nil {
		return err
	}
	if inMode == "1" {
		return o.remoteCommand("send-keys", "-X", scrollCmd)
	}
	return o.remoteCommand("send-keys", key)
}

// toggleCopyMode enters or exits copy-mode on the remote tmux.
func (o *SSHTmuxOperator) toggleCopyMode() error {
	inMode, err := o.remoteOutput("display-message", "-p", "\\#{pane_in_mode}")
	if err != nil {
		return err
	}
	if inMode == "1" {
		return o.remoteCommand("send-keys", "-X", "cancel")
	}
	return o.remoteCommand("copy-mode", "-eu")
}

// remoteOutput runs a tmux command remotely and returns trimmed stdout.
func (o *SSHTmuxOperator) remoteOutput(args ...string) (string, error) {
	socket, host, err := readSSHInfo()
	if err != nil {
		return "", err
	}
	cmdArgs := append([]string{"-S", socket, "-o", "BatchMode=yes", host, "tmux"}, args...)
	cmd := exec.Command("ssh", cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("remote tmux %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func readSSHInfo() (socket, host string, err error) {
	data, err := os.ReadFile(sshInfoFile)
	if err != nil {
		return "", "", fmt.Errorf("ssh info not available: %w", err)
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), " ", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid ssh info file")
	}
	return parts[0], parts[1], nil
}
