package keyboard

// STTProvider transcribes speech into text. The only real implementation is
// RemoteSTTProvider — transcription runs server-side so the keyboard never
// loads a model. The interface is kept for tests/fakes.
type STTProvider interface {
	// Initialize blocks until the backend is ready.
	Initialize() error
	Transcribe(audio []byte, sampleRate int) (string, error)
}
