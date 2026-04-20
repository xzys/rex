//go:build !sherpa

package keyboard

import "time"

// StubSTTProvider returns a fixed string after a short delay. For demo/testing.
type StubSTTProvider struct{}

func (s *StubSTTProvider) Initialize() error { return nil }

func (s *StubSTTProvider) Transcribe(audio []byte, sampleRate int) (string, error) {
	time.Sleep(1 * time.Second)
	return "hello world", nil
}

func defaultSTTProvider(sttMode string) STTProvider {
	_ = sttMode
	return &StubSTTProvider{}
}
