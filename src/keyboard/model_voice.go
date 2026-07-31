package keyboard

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/xrfr/t-rex/v2/keyrepeat"
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

// Bubbletea message types for voice model
type STTResultMsg struct {
	Text string
	Err  error
}
type STTInitDoneMsg struct{ Err error }
type ErrorTimeoutMsg struct{}
type SpinnerTickMsg struct{}
type DeleteTickMsg struct{}
type DPadArrowTickMsg struct {
	Key string
	Gen int
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type VoiceModel struct {
	state        voiceState
	pendingCmd   tea.Cmd
	stt          STTProvider
	errMsg       string
	spinnerFrame int
	deleting     bool
	isCompanion  bool // companion mode: iPhone records; otherwise the daemon records
	heldDPad     int  // -1 when nothing held; guards against stale repeat ticks
	holdGen      int  // bumped on dpad down/up to invalidate stale ticks
}

func NewVoiceModel(provider STTProvider, isCompanion bool) *VoiceModel {
	m := &VoiceModel{
		state:       stateInitializing,
		stt:         provider,
		isCompanion: isCompanion,
		heldDPad:    -1,
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
	case DPadArrowTickMsg:
		if msg.Gen == m.holdGen && m.heldDPad == dpadFromArrow(msg.Key) {
			m.pendingCmd = dpadArrowTick(msg.Key, msg.Gen, keyrepeat.Interval)
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
	return tea.Tick(keyrepeat.Interval, func(time.Time) tea.Msg {
		return DeleteTickMsg{}
	})
}

// HandleKey implements KeyboardModel. Returns nil for all keys — output comes via STTResultMsg.
func (m *VoiceModel) HandleKey(key int, isKeyDown bool) *string {
	switch key {
	case protocol.KeyY:
		if isKeyDown {
			if m.state == stateIdle {
				m.state = stateRecording
				slog.Info("voice: recording started")
				// Tell the server to start recording (daemon mic or companion app)
				if starter, ok := m.stt.(interface{ NotifyStart() }); ok {
					starter.NotifyStart()
				}
			}
		} else {
			if m.state == stateRecording {
				m.state = stateProcessing
				m.spinnerFrame = 0
				slog.Info("voice: recording stopped, waiting for transcription")

				stt := m.stt
				m.pendingCmd = func() tea.Msg {
					text, err := stt.Transcribe(nil, 16000)
					return STTResultMsg{Text: text, Err: err}
				}
			}
		}
	case protocol.KeyB:
		if isKeyDown {
			m.deleting = true
			// First delete is immediate (keyboard.go sends it), then wait
			// Delay before the first repeat tick.
			m.pendingCmd = tea.Tick(keyrepeat.Delay, func(time.Time) tea.Msg {
				return DeleteTickMsg{}
			})
		} else {
			m.deleting = false
		}
	case protocol.KeyDPadUp, protocol.KeyDPadDown, protocol.KeyDPadLeft, protocol.KeyDPadRight:
		arrowKey := dpadToArrowKey(key)
		if isKeyDown {
			m.heldDPad = key
			m.holdGen++
			m.pendingCmd = dpadArrowTick(arrowKey, m.holdGen, keyrepeat.Delay)
			s := arrowKey
			return &s
		} else if m.heldDPad == key {
			m.heldDPad = -1
			m.holdGen++ // invalidate in-flight tick
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
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	recordButton := green
	if !m.isCompanion {
		recordButton = red
	}

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
		status = "HOLD " + recordButton.Render("Y") + " TO RECORD"
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

// --- D-pad arrow repeat helpers ---

func dpadArrowTick(arrowKey string, gen int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return DPadArrowTickMsg{Key: arrowKey, Gen: gen}
	})
}

func dpadToArrowKey(key int) string {
	switch key {
	case protocol.KeyDPadUp:
		return "Up"
	case protocol.KeyDPadDown:
		return "Down"
	case protocol.KeyDPadLeft:
		return "Left"
	case protocol.KeyDPadRight:
		return "Right"
	}
	return ""
}

func dpadFromArrow(arrowKey string) int {
	switch arrowKey {
	case "Up":
		return protocol.KeyDPadUp
	case "Down":
		return protocol.KeyDPadDown
	case "Left":
		return protocol.KeyDPadLeft
	case "Right":
		return protocol.KeyDPadRight
	}
	return -1
}
