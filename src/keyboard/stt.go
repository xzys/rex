package keyboard

// STTProvider transcribes raw PCM audio bytes into text.
type STTProvider interface {
	// Initialize loads the model. Called once, may be slow.
	Initialize() error
	Transcribe(audio []byte, sampleRate int) (string, error)
}
