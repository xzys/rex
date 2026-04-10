//go:build sherpa

package keyboard

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

const defaultSTTModelDir = "/userdata/system/apps/rex/stt"

type SherpaOnnxProvider struct {
	modelDir   string
	recognizer *sherpa.OfflineRecognizer
}

func NewSherpaOnnxProvider(modelDir string) (*SherpaOnnxProvider, error) {
	modelPath := filepath.Join(modelDir, "model.int8.onnx")
	tokensPath := filepath.Join(modelDir, "tokens.txt")

	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("model not found: %s", modelPath)
	}
	if _, err := os.Stat(tokensPath); err != nil {
		return nil, fmt.Errorf("tokens not found: %s", tokensPath)
	}

	return &SherpaOnnxProvider{modelDir: modelDir}, nil
}

func (p *SherpaOnnxProvider) Initialize() error {
	if p.recognizer != nil {
		return nil
	}

	modelPath := filepath.Join(p.modelDir, "model.int8.onnx")
	tokensPath := filepath.Join(p.modelDir, "tokens.txt")

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
	return result.Text, nil
}

func defaultSTTProvider() STTProvider {
	modelDir := os.Getenv("REX_STT_MODEL_DIR")
	if modelDir == "" {
		modelDir = defaultSTTModelDir
	}

	provider, err := NewSherpaOnnxProvider(modelDir)
	if err != nil {
		slog.Error("failed to load sherpa-onnx", "err", err)
		os.Exit(1)
	}
	return provider
}
