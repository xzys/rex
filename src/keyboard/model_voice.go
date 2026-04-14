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
	stateInitializing voiceState = iota
	stateIdle
	stateRecording
	stateProcessing
	stateError
)

const deleteRepeatMs = 50

// Bubbletea message types for voice model
type STTResultMsg struct {
	Text string
	Err  error
}
type STTInitDoneMsg struct{ Err error }
type ErrorTimeoutMsg struct{}
type SpinnerTickMsg struct{}
type DeleteTickMsg struct{}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type VoiceModel struct {
	state        voiceState
	pendingCmd   tea.Cmd
	stt          STTProvider
	audio        *AudioCapture // nil in companion mode (iPhone captures audio)
	errMsg       string
	spinnerFrame int
	deleting     bool
	isCompanion  bool
}

func NewVoiceModel(provider STTProvider) *VoiceModel {
	_, isCompanion := provider.(*CompanionSTTProvider)
	m := &VoiceModel{
		state:       stateInitializing,
		stt:         provider,
		isCompanion: isCompanion,
	}
	if !isCompanion {
		m.audio = NewAudioCapture()
	}
	// Queue async initialization — keyboard.go will drain this
	stt := m.stt
	m.pendingCmd = tea.Batch(
		func() tea.Msg {
			err := stt.Initialize()
			return STTInitDoneMsg{Err: err}
		},
		m.spinnerTick(),
	)
	return m
}

// STTProvider returns the STT provider used by this voice model.
func (m *VoiceModel) STTProvider() STTProvider {
	return m.stt
}

// DrainCmd returns and clears any pending tea.Cmd queued by HandleKey or HandleMsg.
func (m *VoiceModel) DrainCmd() tea.Cmd {
	cmd := m.pendingCmd
	m.pendingCmd = nil
	return cmd
}

// HandleMsg processes async bubbletea messages.
func (m *VoiceModel) HandleMsg(msg tea.Msg) {
	switch msg := msg.(type) {
	case STTInitDoneMsg:
		if msg.Err != nil {
			slog.Error("stt init failed", "err", msg.Err)
			m.setError(msg.Err.Error())
			return
		}
		m.state = stateIdle
		slog.Info("stt initialized")
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
		m.state = stateIdle
	case SpinnerTickMsg:
		if m.state == stateInitializing {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(spinnerFrames)
			m.pendingCmd = m.spinnerTick()
		}
	case DeleteTickMsg:
		// keyboard.go handles sending the actual delete key;
		// we just queue the next tick if still held
		if m.deleting {
			m.pendingCmd = m.deleteTick()
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

func (m *VoiceModel) deleteTick() tea.Cmd {
	return tea.Tick(deleteRepeatMs*time.Millisecond, func(time.Time) tea.Msg {
		return DeleteTickMsg{}
	})
}

// HandleKey implements KeyboardModel. Returns nil for all keys — output comes via STTResultMsg.
func (m *VoiceModel) HandleKey(key int, isKeyDown bool) *string {
	switch key {
	case protocol.KeyY:
		if isKeyDown {
			if m.state == stateIdle {
				if m.audio != nil {
					if err := m.audio.Start(); err != nil {
						slog.Error("failed to start recording", "err", err)
						m.setError("mic failed")
						return nil
					}
				}
				m.state = stateRecording
				slog.Info("voice: recording started")
				// Notify companion app to start recording (no-op for sherpa/stub)
				if starter, ok := m.stt.(interface{ NotifyStart() }); ok {
					starter.NotifyStart()
				}
			}
		} else {
			if m.state == stateRecording {
				var pcm []byte
				if m.audio != nil {
					pcm = m.audio.Stop()
				}
				m.state = stateProcessing
				m.spinnerFrame = 0
				slog.Info("voice: recording stopped, starting STT", "pcm_bytes", len(pcm))

				stt := m.stt
				m.pendingCmd = func() tea.Msg {
					text, err := stt.Transcribe(pcm, 16000)
					return STTResultMsg{Text: text, Err: err}
				}
			}
		}
	case protocol.KeyB:
		if isKeyDown {
			m.deleting = true
			// First delete is immediate (keyboard.go sends it), then start repeat
			m.pendingCmd = m.deleteTick()
		} else {
			m.deleting = false
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
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	var status string
	switch m.state {
	case stateInitializing:
		frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
		if m.isCompanion {
			status = fmt.Sprintf("WAITING FOR CONNECTION... %s", frame)
		} else {
			status = fmt.Sprintf("INITIALIZING %s", frame)
		}
	case stateIdle:
		status = "HOLD " + red.Render("Y") + " TO RECORD"
	case stateRecording:
		status = "RECORDING"
	case stateProcessing:
		status = dim.Render("PROCESSING")
	case stateError:
		status = red.Render(fmt.Sprintf("ERROR: %s", m.errMsg))
	}

	left := dim.Render("B DELETE")
	right := dim.Render("X EXIT")
	statusW := lipgloss.Width(status)
	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)

	// Center status between left and right hints
	centerPos := (width - statusW) / 2
	leftGap := centerPos - leftW
	if leftGap < 1 {
		leftGap = 1
	}
	rightGap := width - centerPos - statusW - rightW
	if rightGap < 1 {
		rightGap = 1
	}

	return left + strings.Repeat(" ", leftGap) + status + strings.Repeat(" ", rightGap) + right
}
