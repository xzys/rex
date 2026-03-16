package keyboard

import "time"

// STTProvider transcribes raw PCM audio bytes into text.
type STTProvider interface {
	Transcribe(audio []byte, sampleRate int) (string, error)
}

// StubSTTProvider returns a fixed string after a short delay. For Phase 1 demo.
type StubSTTProvider struct{}

func (s *StubSTTProvider) Transcribe(audio []byte, sampleRate int) (string, error) {
	time.Sleep(1 * time.Second)
	return "hello world", nil
}
