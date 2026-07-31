//go:build !sherpa

package stt

import "time"

// StubProvider returns a fixed string after a short delay. For demo/testing.
type StubProvider struct{}

func (s *StubProvider) Initialize() error { return nil }

func (s *StubProvider) Transcribe(audio []byte, sampleRate int) (string, error) {
	time.Sleep(1 * time.Second)
	return "hello world", nil
}

func (s *StubProvider) Close() {}

func NewProvider(sttMode string) Provider {
	_ = sttMode
	return &StubProvider{}
}
