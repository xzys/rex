package server

import (
	"fmt"
	"os"
	"strings"

	"github.com/xrfr/t-rex/v2/protocol"
)

const sshInfoFile = "/tmp/rex-ssh-info"

type SSHTmuxOperator struct {
	l2Down     bool
	r2Down     bool
	comboFired bool // prevents the trailing key-up from firing a pane action after a window combo
}

func NewSSHTmuxOperator() *SSHTmuxOperator {
	return &SSHTmuxOperator{}
}

func (o *SSHTmuxOperator) HandleInput(key int, isDown bool) (bool, error) {
	switch key {
	case protocol.KeyL2:
		o.l2Down = isDown
		if isDown {
			if o.r2Down {
				o.comboFired = true
				return true, o.remoteCommand("previous-window")
			}
		} else {
			if !o.r2Down && !o.comboFired {
				return true, o.remoteCommand("select-pane", "-t", ":.-")
			}
			o.comboFired = false
		}
		return true, nil

	case protocol.KeyR2:
		o.r2Down = isDown
		if isDown {
			if o.l2Down {
				o.comboFired = true
				return true, o.remoteCommand("next-window")
			}
		} else {
			if !o.l2Down && !o.comboFired {
				return true, o.remoteCommand("select-pane", "-t", ":.+")
			}
			o.comboFired = false
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
