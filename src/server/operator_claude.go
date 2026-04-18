package server

import (
	"log/slog"
	"sync"
	"time"

	"github.com/xrfr/t-rex/v2/protocol"
)

type ClaudeOperator struct {
	mu          sync.Mutex
	voiceCancel chan struct{}
	voiceRelay  *audioRelay
	bsCancel    chan struct{}
}

const (
	claudeBackspaceDelay    = 400 * time.Millisecond
	claudeBackspaceInterval = 50 * time.Millisecond
	claudeVoiceInterval     = 30 * time.Millisecond
)

func NewClaudeOperator() *ClaudeOperator {
	return &ClaudeOperator{}
}

func (o *ClaudeOperator) HandleInput(key int, isDown bool) (bool, error) {
	switch key {
	case protocol.KeyY:
		if isDown {
			o.startVoice()
			return true, nil
		}
		o.stopVoice()
		return true, nil
	case protocol.KeyA:
		if isDown {
			return true, execute("tmux", "send-keys", "-t", "0", "Enter")
		}
		return true, nil
	case protocol.KeyX:
		if isDown {
			return true, execute("tmux", "send-keys", "-t", "0", "Escape")
		}
		return true, nil
	case protocol.KeyB:
		if isDown {
			return true, o.startBackspace()
		}
		o.stopBackspace()
		return true, nil
	case protocol.KeyDPadUp:
		if isDown {
			return true, execute("tmux", "copy-mode", "-eu", "-t", "0")
		}
		return true, nil
	case protocol.KeyDPadDown:
		if isDown {
			return true, tryExecute("tmux", []string{"not in a mode"}, "send-keys", "-t", "0", "-X", "page-down")
		}
		return true, nil
	case protocol.KeyDPadLeft:
		if isDown {
			return true, execute("tmux", "send-keys", "-t", "0", "Left")
		}
		return true, nil
	case protocol.KeyDPadRight:
		if isDown {
			return true, execute("tmux", "send-keys", "-t", "0", "Right")
		}
		return true, nil
	}
	return false, nil
}

// Manufacture key-repeat for claude's /voice PTT — SDL joystick events don't auto-repeat.
// If pane 0 is SSH'd, also start an audio relay so the remote's default ALSA
// source receives the Brick's mic. See notes/plans/cross-device-mic-relay.md.
func (o *ClaudeOperator) startVoice() {
	o.mu.Lock()
	if o.voiceCancel != nil {
		close(o.voiceCancel)
	}
	cancel := make(chan struct{})
	o.voiceCancel = cancel

	if remote, err := detectSSHPane("0"); err != nil {
		slog.Warn("claude: ssh pane detect failed", "err", err)
	} else if remote != nil {
		if relay, err := startAudioRelay(*remote); err != nil {
			slog.Warn("claude: audio relay start failed", "err", err, "remote", remote.Addr())
		} else {
			o.voiceRelay = relay
		}
	}
	o.mu.Unlock()

	go func() {
		ticker := time.NewTicker(claudeVoiceInterval)
		defer ticker.Stop()
		for {
			select {
			case <-cancel:
				return
			case <-ticker.C:
				if err := execute("tmux", "send-keys", "-t", "0", "Space"); err != nil {
					slog.Warn("claude: voice space failed", "err", err)
					return
				}
			}
		}
	}()
}

func (o *ClaudeOperator) stopVoice() {
	o.mu.Lock()
	relay := o.voiceRelay
	o.voiceRelay = nil
	if o.voiceCancel != nil {
		close(o.voiceCancel)
		o.voiceCancel = nil
	}
	o.mu.Unlock()
	relay.Stop()
}

func (o *ClaudeOperator) startBackspace() error {
	if err := execute("tmux", "send-keys", "-t", "0", "BSpace"); err != nil {
		return err
	}
	o.mu.Lock()
	if o.bsCancel != nil {
		close(o.bsCancel)
	}
	cancel := make(chan struct{})
	o.bsCancel = cancel
	o.mu.Unlock()

	go func() {
		select {
		case <-cancel:
			return
		case <-time.After(claudeBackspaceDelay):
		}
		ticker := time.NewTicker(claudeBackspaceInterval)
		defer ticker.Stop()
		for {
			select {
			case <-cancel:
				return
			case <-ticker.C:
				if err := execute("tmux", "send-keys", "-t", "0", "BSpace"); err != nil {
					slog.Warn("claude: backspace repeat failed", "err", err)
					return
				}
			}
		}
	}()
	return nil
}

func (o *ClaudeOperator) stopBackspace() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.bsCancel != nil {
		close(o.bsCancel)
		o.bsCancel = nil
	}
}
