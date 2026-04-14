package keyboard

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// CompanionSTTProvider implements STTProvider by routing through the
// WebSocket server to the iOS companion app. It sends voice commands
// via a callback (which writes to the Unix socket) and waits for the
// final transcription to be delivered via FinalCh.
type CompanionSTTProvider struct {
	// SendVoiceCmd sends a VoiceCommandMessage to the server.
	// action is "start", "stop", or "status".
	SendVoiceCmd func(action string) error

	// FinalCh receives the final transcription text from the server
	// (delivered by keyboard.go when it receives a VoiceTextMessage).
	FinalCh chan string

	// ConnectedCh receives companion connection status from keyboard.go
	// (delivered when it receives a CompanionStatusMessage).
	ConnectedCh chan bool
}

// NewCompanionSTTProvider creates a new companion STT provider.
// sendVoiceCmd is called to send voice commands to the server.
func NewCompanionSTTProvider(sendVoiceCmd func(action string) error) *CompanionSTTProvider {
	return &CompanionSTTProvider{
		SendVoiceCmd: sendVoiceCmd,
		FinalCh:      make(chan string, 1),
		ConnectedCh:  make(chan bool, 1),
	}
}

// Initialize blocks until the companion app is connected.
// It sends a "status" query to the server and waits for a
// CompanionStatusMessage with Connected=true.
func (p *CompanionSTTProvider) Initialize() error {
	if err := p.SendVoiceCmd("status"); err != nil {
		return fmt.Errorf("companion: failed to query status: %w", err)
	}
	for {
		connected := <-p.ConnectedCh
		if connected {
			return nil
		}
	}
}

// NotifyStart sends a start command to the server, which forwards it
// to the iOS companion app.
func (p *CompanionSTTProvider) NotifyStart() {
	if err := p.SendVoiceCmd("start"); err != nil {
		slog.Error("companion: failed to send start", "err", err)
	}
}

// Transcribe sends a stop command and blocks until the final transcription
// arrives from the iOS companion app via the server.
// The audio parameter is ignored — the iOS app captures its own audio.
func (p *CompanionSTTProvider) Transcribe(audio []byte, sampleRate int) (string, error) {
	if err := p.SendVoiceCmd("stop"); err != nil {
		return "", fmt.Errorf("companion: failed to send stop: %w", err)
	}

	// Wait for final transcription with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	select {
	case text := <-p.FinalCh:
		return text, nil
	case <-ctx.Done():
		return "", fmt.Errorf("companion: timed out waiting for transcription")
	}
}
