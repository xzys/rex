package keyboard

// STTProvider transcribes raw PCM audio bytes into text.
type STTProvider interface {
	// Initialize loads the model. Called once, may be slow.
	Initialize() error
	Transcribe(audio []byte, sampleRate int) (string, error)
}

// DefaultSTTProvider returns the on-device provider for the given mode
// (sherpa-onnx in sherpa builds, a stub elsewhere). Companion mode is not
// handled here — callers that support it wire up CompanionSTTProvider themselves.
func DefaultSTTProvider(sttMode string) STTProvider {
	return defaultSTTProvider(sttMode)
}
