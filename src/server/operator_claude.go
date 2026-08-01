package server

import (
	"log/slog"
	"sync"

	"github.com/xrfr/t-rex/v2/keyrepeat"
	"github.com/xrfr/t-rex/v2/protocol"
)

type ClaudeOperator struct {
	mu            sync.Mutex
	voiceCancel   chan struct{}
	voiceRelay    *audioRelay
	voiceRepeater *keyrepeat.KeyRepeater
	bsRepeater    *keyrepeat.KeyRepeater
	dpadRepeater  *keyrepeat.KeyRepeater
}

func NewClaudeOperator() *ClaudeOperator {
	return &ClaudeOperator{}
}

func (o *ClaudeOperator) RepeatKeys() []int { return nil }

func (o *ClaudeOperator) HandleInput(key int, isDown bool) (bool, error) {
	switch key {
	case protocol.KeyY:
		if isDown {
			o.startVoice()
			return true, nil
		}
		o.stopVoice()
		return true, nil
	case protocol.KeyX:
		if isDown {
			return true, execute("tmux", "send-keys", "Escape")
		}
		return true, nil
	case protocol.KeyB:
		if isDown {
			return true, o.startBackspace()
		}
		o.stopBackspace()
		return true, nil
	case protocol.KeyDPadUp, protocol.KeyDPadDown, protocol.KeyDPadLeft, protocol.KeyDPadRight:
		arrowKey := dpadToArrow(key)
		if isDown {
			if err := execute("tmux", "send-keys", arrowKey); err != nil {
				return true, err
			}
			o.startDpadRepeat(key, arrowKey)
		} else {
			o.stopDpadRepeat(key)
		}
		return true, nil
	case protocol.KeyR3:
		if isDown {
			return true, execute("tmux", "send-keys", "/")
		}
		return true, nil
	}
	return false, nil
}

// Manufacture key-repeat for claude's /voice PTT — SDL joystick events don't auto-repeat.
// If the focused pane is SSH'd, also start an audio relay so the remote's default
// ALSA source receives the Brick's mic. See notes/plans/cross-device-mic-relay.md.
func (o *ClaudeOperator) startVoice() {
	o.mu.Lock()
	if o.voiceCancel != nil {
		close(o.voiceCancel)
	}
	cancel := make(chan struct{})
	o.voiceCancel = cancel

	if remote, err := detectSSHPane(""); err != nil {
		slog.Warn("claude: ssh pane detect failed", "err", err)
	} else if remote != nil {
		if relay, err := startAudioRelay(*remote); err != nil {
			slog.Warn("claude: audio relay start failed", "err", err, "remote", remote.Addr())
		} else {
			o.voiceRelay = relay
		}
	}
	if o.voiceRepeater != nil {
		o.voiceRepeater.StopAll()
	}
	o.voiceRepeater = keyrepeat.NewKeyRepeater(
		func(k int) error {
			return execute("tmux", "send-keys", "Space")
		},
		0, // no initial delay — PTT starts immediately
		keyrepeat.PTTInterval,
	)
	o.voiceRepeater.Start(protocol.KeyY)
	o.mu.Unlock()
}

func (o *ClaudeOperator) stopVoice() {
	o.mu.Lock()
	relay := o.voiceRelay
	o.voiceRelay = nil
	if o.voiceCancel != nil {
		close(o.voiceCancel)
		o.voiceCancel = nil
	}
	if o.voiceRepeater != nil {
		o.voiceRepeater.StopAll()
	}
	o.mu.Unlock()
	relay.Stop()
}

func (o *ClaudeOperator) startBackspace() error {
	// First delete fires immediately.
	if err := execute("tmux", "send-keys", "BSpace"); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.bsRepeater != nil {
		o.bsRepeater.StopAll()
	}
	o.bsRepeater = keyrepeat.NewKeyRepeater(
		func(k int) error {
			return execute("tmux", "send-keys", "BSpace")
		},
		keyrepeat.Delay,
		keyrepeat.Interval,
	)
	o.bsRepeater.Start(protocol.KeyB)
	return nil
}

func (o *ClaudeOperator) stopBackspace() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.bsRepeater != nil {
		o.bsRepeater.StopAll()
	}
}

// --- helpers ---

func dpadToArrow(key int) string {
	switch key {
	case protocol.KeyDPadUp:
		return "Up"
	case protocol.KeyDPadDown:
		return "Down"
	case protocol.KeyDPadLeft:
		return "Left"
	case protocol.KeyDPadRight:
		return "Right"
	}
	return ""
}

func (o *ClaudeOperator) startDpadRepeat(key int, arrowKey string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.dpadRepeater != nil {
		o.dpadRepeater.StopAll()
	}
	o.dpadRepeater = keyrepeat.NewKeyRepeater(
		func(k int) error {
			return execute("tmux", "send-keys", arrowKey)
		},
		keyrepeat.Delay,
		keyrepeat.Interval,
	)
	o.dpadRepeater.Start(key)
}

func (o *ClaudeOperator) stopDpadRepeat(key int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.dpadRepeater != nil {
		o.dpadRepeater.Stop(key)
	}
}
