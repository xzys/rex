package keyboard

// STTProvider transcribes raw PCM audio bytes into text.
type STTProvider interface {
	Transcribe(audio []byte, sampleRate int) (string, error)
}
