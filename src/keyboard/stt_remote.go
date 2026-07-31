package keyboard

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// RemoteSTTProvider implements STTProvider by routing through the rex
// daemon over the Unix socket. The daemon either runs the resident
// sherpa-onnx recognizer (parakeet/sherpa modes) or relays to the iOS
// companion app (companion mode). Audio is captured by the daemon (or the
// iPhone), never by the keyboard — so the keyboard starts instantly and
// never loads a model.
type RemoteSTTProvider struct {
	// SendVoiceCmd sends a VoiceCommandMessage to the server.
	// action is "start", "stop", or "status".
	SendVoiceCmd func(action string) error

	// FinalCh receives the final transcription text from the server
	// (delivered by keyboard.go when it receives a VoiceTextMessage).
	FinalCh chan string

	// ReadyCh receives STT readiness from the server: model loaded, or
	// companion app connected (delivered by keyboard.go when it receives
	// a CompanionStatusMessage).
	ReadyCh chan bool
}

// NewRemoteSTTProvider creates a new remote STT provider.
// sendVoiceCmd is called to send voice commands to the server.
func NewRemoteSTTProvider(sendVoiceCmd func(action string) error) *RemoteSTTProvider {
	return &RemoteSTTProvider{
		SendVoiceCmd: sendVoiceCmd,
		FinalCh:      make(chan string, 1),
		ReadyCh:      make(chan bool, 1),
	}
}

// Initialize blocks until the server reports its STT backend ready.
// It sends a "status" query and waits for a CompanionStatusMessage with
// Connected=true — sent immediately if the model is already loaded, or
// broadcast by the server once loading finishes.
func (p *RemoteSTTProvider) Initialize() error {
	if err := p.SendVoiceCmd("status"); err != nil {
		return fmt.Errorf("remote stt: failed to query status: %w", err)
	}
	for {
		ready := <-p.ReadyCh
		if ready {
			return nil
		}
	}
}

// NotifyStart sends a start command to the server, which begins recording
// (or forwards it to the iOS companion app).
func (p *RemoteSTTProvider) NotifyStart() {
	// Drop any stale final from a previous session (e.g. an empty result
	// delivered after a timeout) so the next Transcribe blocks on the real
	// answer instead of returning yesterday's leftovers.
	select {
	case <-p.FinalCh:
	default:
	}
	if err := p.SendVoiceCmd("start"); err != nil {
		slog.Error("remote stt: failed to send start", "err", err)
	}
}

// Transcribe sends a stop command and blocks until the final transcription
// arrives from the server. The audio parameter is ignored — the server
// captures its own audio.
func (p *RemoteSTTProvider) Transcribe(audio []byte, sampleRate int) (string, error) {
	if err := p.SendVoiceCmd("stop"); err != nil {
		return "", fmt.Errorf("remote stt: failed to send stop: %w", err)
	}

	// Wait for final transcription with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	select {
	case text := <-p.FinalCh:
		return text, nil
	case <-ctx.Done():
		return "", fmt.Errorf("remote stt: timed out waiting for transcription")
	}
}
