// Package stt provides speech-to-text for the rex daemon: mic capture plus
// a resident sherpa-onnx recognizer. Living in the long-running daemon means
// the keyboard doesn't pay the model load on every launch — it drives STT
// over the voice_command/voice_text socket protocol instead.
package stt

// Provider transcribes raw PCM audio bytes into text.
type Provider interface {
	// Initialize loads the model. Called once, may be slow.
	Initialize() error
	Transcribe(audio []byte, sampleRate int) (string, error)
	// Close frees the loaded model.
	Close()
}
