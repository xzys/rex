package server

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/xrfr/t-rex/v2/stt"
)

// sttEngine owns the resident STT model and mic capture inside the daemon,
// so the keyboard doesn't reload the model on every launch. The keyboard
// drives it over the existing voice_command/voice_text protocol.
//
// TODO: run the recognizer as a small supervised subprocess the daemon can
// restart, so a crash in the CGo/onnx stack can't take down input handling.
type sttEngine struct {
	mu       sync.Mutex
	mode     string
	provider stt.Provider
	ready    bool
	loading  bool
	capture  *stt.AudioCapture

	// decodeMu serializes Transcribe calls — the keyboard and the dictator
	// socket can request recognition concurrently, and the CGo decoder isn't
	// guaranteed thread-safe.
	decodeMu sync.Mutex

	// onReady is called (outside the lock) when a model finishes loading,
	// so the server can unblock a keyboard waiting in Initialize().
	onReady func()
}

func newSTTEngine() *sttEngine {
	return &sttEngine{capture: stt.NewAudioCapture()}
}

// Ensure makes sure the model for the given mode is loaded or loading.
// Loading happens in a background goroutine; Ready() flips when it's done.
// Switching modes (parakeet ↔ sherpa) frees the old model and loads the new.
func (e *sttEngine) Ensure(mode string) {
	e.mu.Lock()
	if e.mode == mode && (e.ready || e.loading) {
		e.mu.Unlock()
		return
	}
	old := e.provider
	p := stt.NewProvider(mode)
	e.mode = mode
	e.provider = p
	e.ready = false
	e.loading = true
	e.mu.Unlock()

	go func() {
		if old != nil {
			old.Close()
		}
		err := p.Initialize()

		e.mu.Lock()
		if e.provider != p {
			// Mode switched again while this model was loading; discard it.
			e.mu.Unlock()
			p.Close()
			return
		}
		e.loading = false
		e.ready = err == nil
		cb := e.onReady
		e.mu.Unlock()

		if err != nil {
			slog.Error("stt: model load failed", "mode", mode, "err", err)
			return
		}
		slog.Info("stt: model loaded", "mode", mode)
		if cb != nil {
			cb()
		}
	}()
}

func (e *sttEngine) Ready() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ready
}

// StartRecording begins mic capture. The keyboard only sends "start" after
// readiness was reported, so failing here when the model isn't loaded is an
// edge case (e.g. mode switched mid-session), not the normal path.
func (e *sttEngine) StartRecording() error {
	if !e.Ready() {
		return fmt.Errorf("stt: model not loaded")
	}
	return e.capture.Start()
}

// StopAndTranscribe stops mic capture and runs the recognizer on the
// captured audio. Blocking — call from a goroutine.
func (e *sttEngine) StopAndTranscribe() (string, error) {
	return e.Transcribe(e.capture.Stop(), 16000)
}

// Transcribe runs the recognizer on caller-supplied PCM (e.g. dictator
// segments arriving over the stt socket). Blocking — call from a goroutine.
func (e *sttEngine) Transcribe(pcm []byte, sampleRate int) (string, error) {
	e.mu.Lock()
	p := e.provider
	ready := e.ready
	e.mu.Unlock()
	if p == nil || !ready {
		return "", fmt.Errorf("stt: model not loaded")
	}
	e.decodeMu.Lock()
	defer e.decodeMu.Unlock()
	return p.Transcribe(pcm, sampleRate)
}
