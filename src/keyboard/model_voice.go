package keyboard

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xrfr/t-rex/v2/protocol"
)

type voiceState int

const (
	stateIdle voiceState = iota
	stateRecording
	stateProcessing
	stateError
)

// Bubbletea message types for voice model
type AudioLevelMsg struct{ Level float32 }
type STTResultMsg struct {
	Text string
	Err  error
}
type ErrorTimeoutMsg struct{}
type SpinnerTickMsg struct{}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type VoiceModel struct {
	state        voiceState
	pendingCmd   tea.Cmd
	stt          STTProvider
	audio        *AudioCapture
	errMsg       string
	spinnerFrame int
}

func NewVoiceModel(provider STTProvider) *VoiceModel {
	return &VoiceModel{
		state: stateIdle,
		stt:   provider,
		audio: NewAudioCapture(),
	}
}

// DrainCmd returns and clears any pending tea.Cmd queued by HandleKey or HandleMsg.
func (m *VoiceModel) DrainCmd() tea.Cmd {
	cmd := m.pendingCmd
	m.pendingCmd = nil
	return cmd
}

// HandleMsg processes async bubbletea messages (STT results, spinner ticks, error timeouts).
func (m *VoiceModel) HandleMsg(msg tea.Msg) {
	switch msg := msg.(type) {
	case STTResultMsg:
		if msg.Err != nil {
			slog.Error("stt error", "err", msg.Err)
			m.setError(msg.Err.Error())
			return
		}
		if strings.TrimSpace(msg.Text) == "" {
			m.setError("no speech detected")
			return
		}
		// Success — keyboard.go will send the text via sendString
		m.state = stateIdle
	case SpinnerTickMsg:
		if m.state == stateProcessing {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			m.pendingCmd = m.spinnerTick()
		}
	case ErrorTimeoutMsg:
		if m.state == stateError {
			m.state = stateIdle
			m.errMsg = ""
		}
	}
}

func (m *VoiceModel) setError(msg string) {
	m.state = stateError
	m.errMsg = msg
	m.pendingCmd = tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return ErrorTimeoutMsg{}
	})
}

func (m *VoiceModel) spinnerTick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
		return SpinnerTickMsg{}
	})
}

// HandleKey implements KeyboardModel. Returns nil for all keys — output comes via STTResultMsg.
func (m *VoiceModel) HandleKey(key int, isKeyDown bool) *string {
	switch key {
	case protocol.KeyA:
		if isKeyDown {
			if m.state == stateIdle {
				if err := m.audio.Start(); err != nil {
					slog.Error("failed to start recording", "err", err)
					m.setError("mic failed")
					return nil
				}
				m.state = stateRecording
				slog.Info("voice: recording started")
			}
		} else {
			if m.state == stateRecording {
				pcm := m.audio.Stop()
				m.state = stateProcessing
				m.spinnerFrame = 0
				slog.Info("voice: recording stopped, starting STT", "pcm_bytes", len(pcm))

				stt := m.stt
				m.pendingCmd = tea.Batch(
					func() tea.Msg {
						text, err := stt.Transcribe(pcm, 16000)
						return STTResultMsg{Text: text, Err: err}
					},
					m.spinnerTick(),
				)
			}
		}
	}
	return nil
}

// RenderKey implements KeyboardModel (unused for voice — VoiceRenderer takes over).
func (m *VoiceModel) RenderKey(i, j int, key string) (string, string, string) {
	return "7", "0", key
}

// RenderView implements VoiceRenderer.
func (m *VoiceModel) RenderView(width int) string {
	var text string
	switch m.state {
	case stateIdle:
		text = "HOLD A TO RECORD"
	case stateRecording:
		text = "RECORDING  ●"
	case stateProcessing:
		frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
		text = fmt.Sprintf("PROCESSING %s", frame)
	case stateError:
		text = fmt.Sprintf("ERROR: %s", m.errMsg)
	}

	style := lipgloss.NewStyle().
		Width(width).
		Align(lipgloss.Center).
		Padding(1, 0)

	return style.Render(text)
}
