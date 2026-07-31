//go:build sherpa

package stt

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"

	cfg "github.com/xrfr/t-rex/v2/config"
)

const (
	modelDirSherpa   = "/userdata/system/apps/rex/stt"
	modelDirParakeet = "/userdata/system/apps/rex/stt-parakeet"
)

type SherpaOnnxProvider struct {
	modelDir   string
	recognizer *sherpa.OfflineRecognizer
}

func NewSherpaOnnxProvider(modelDir string) *SherpaOnnxProvider {
	return &SherpaOnnxProvider{modelDir: modelDir}
}

func (p *SherpaOnnxProvider) Initialize() error {
	if p.recognizer != nil {
		return nil
	}

	modelPath := filepath.Join(p.modelDir, "model.int8.onnx")
	tokensPath := filepath.Join(p.modelDir, "tokens.txt")

	if _, err := os.Stat(modelPath); err != nil {
		return fmt.Errorf("model not found: %s", modelPath)
	}
	if _, err := os.Stat(tokensPath); err != nil {
		return fmt.Errorf("tokens not found: %s", tokensPath)
	}

	config := sherpa.OfflineRecognizerConfig{}
	config.ModelConfig.NemoCTC.Model = modelPath
	config.ModelConfig.Tokens = tokensPath
	config.ModelConfig.NumThreads = 4
	config.ModelConfig.Provider = "cpu"

	slog.Info("loading sherpa-onnx recognizer", "model", modelPath)
	r := sherpa.NewOfflineRecognizer(&config)
	if r == nil {
		return fmt.Errorf("failed to create recognizer")
	}
	p.recognizer = r
	slog.Info("sherpa-onnx recognizer loaded")
	return nil
}

func (p *SherpaOnnxProvider) Transcribe(audio []byte, sampleRate int) (string, error) {
	if p.recognizer == nil {
		return "", fmt.Errorf("recognizer not initialized")
	}

	if len(audio) < 2 {
		return "", fmt.Errorf("audio too short")
	}

	stream := sherpa.NewOfflineStream(p.recognizer)
	defer sherpa.DeleteOfflineStream(stream)

	numSamples := len(audio) / 2
	floats := make([]float32, numSamples)
	for i := 0; i < numSamples; i++ {
		sample := int16(binary.LittleEndian.Uint16(audio[i*2 : i*2+2]))
		floats[i] = float32(sample) / 32768.0
	}

	stream.AcceptWaveform(sampleRate, floats)
	p.recognizer.Decode(stream)
	result := stream.GetResult()
	// GetResult returns nil when the decoder produced zero tokens (e.g. silence
	// / no speech). Dereferencing it here used to segfault and take down the
	// whole keyboard. Treat it as an empty transcription — the voice model
	// surfaces that as "no speech detected".
	if result == nil {
		return "", nil
	}
	return result.Text, nil
}

func (p *SherpaOnnxProvider) Close() {
	if p.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(p.recognizer)
		p.recognizer = nil
	}
}

// NewProvider returns the recognizer for the given stt_mode
// ("parakeet" or "sherpa"). Companion mode never reaches here.
func NewProvider(sttMode string) Provider {
	modelDir := os.Getenv("REX_STT_MODEL_DIR")
	if modelDir == "" {
		switch sttMode {
		case cfg.STTModeParakeet:
			modelDir = modelDirParakeet
		default:
			modelDir = modelDirSherpa
		}
	}
	return NewSherpaOnnxProvider(modelDir)
}
