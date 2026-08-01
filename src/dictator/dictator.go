package dictator

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	cfg "github.com/xrfr/t-rex/v2/config"
	"github.com/xrfr/t-rex/v2/keyboard"
)

type state int

const (
	stateInitializing state = iota
	stateRecording
	statePaused
	stateFinishing // waiting for queued transcriptions before quitting
	stateError
)

const drainInterval = 200 * time.Millisecond

type initDoneMsg struct{ err error }
type tickMsg struct{}
type resultMsg struct {
	text string
	err  error
}

// ASCII only — termsp's font doesn't render braille spinner glyphs.
var spinnerFrames = []string{"|", "/", "-", "\\"}

type Model struct {
	stt   keyboard.STTProvider
	audio *keyboard.AudioCapture
	seg   *segmenter

	file *os.File
	path string

	state    state
	errMsg   string
	lines    []string
	queue    [][]byte // segments awaiting transcription
	busy     bool     // a Transcribe call is in flight
	recBytes int      // total PCM captured, for elapsed display
	frame    int
	width    int
	height   int
}

// Run opens (or creates) the output file and runs the dictation TUI.
// Transcribed utterances are appended one per line as they complete.
func Run(sttMode, outPath string) error {
	if outPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		outPath = filepath.Join(home, "dictations", time.Now().Format("2006-01-02_15-04-05")+".txt")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Companion STT needs the iPhone app driving the mic — not usable here.
	if sttMode == cfg.STTModeCompanion {
		sttMode = cfg.STTModeParakeet
	}

	m := &Model{
		stt:   keyboard.DefaultSTTProvider(sttMode),
		audio: keyboard.NewAudioCapture(),
		seg:   newSegmenter(),
		file:  file,
		path:  outPath,
		state: stateInitializing,
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func (m *Model) Init() tea.Cmd {
	stt := m.stt
	return tea.Batch(
		func() tea.Msg { return initDoneMsg{err: stt.Initialize()} },
		tick(),
	)
}

func tick() tea.Cmd {
	return tea.Tick(drainInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case initDoneMsg:
		if msg.err != nil {
			slog.Error("dictator: stt init failed", "err", msg.err)
			m.state = stateError
			m.errMsg = msg.err.Error()
			return m, nil
		}
		return m, m.startRecording()

	case tickMsg:
		m.frame++
		if m.state == stateRecording {
			pcm := m.audio.Drain()
			m.recBytes += len(pcm)
			if seg := m.seg.Feed(pcm); seg != nil {
				return m, m.enqueue(seg)
			}
		}
		return m, tick()

	case resultMsg:
		m.busy = false
		if msg.err != nil {
			slog.Error("dictator: transcribe failed", "err", msg.err)
			m.appendLine("[error: " + msg.err.Error() + "]")
		} else if text := strings.TrimSpace(msg.text); text != "" {
			m.writeLine(text)
		}
		if len(m.queue) > 0 {
			next := m.queue[0]
			m.queue = m.queue[1:]
			return m, m.transcribe(next)
		}
		if m.state == stateFinishing {
			return m, tea.Quit
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	// B on the gamepad arrives as Escape via the global operator fallback.
	case "ctrl+c", "q", "esc":
		return m, m.shutdown()
	// A on the gamepad arrives as Enter.
	case " ", "space", "enter":
		switch m.state {
		case stateRecording:
			return m, m.pause()
		case statePaused:
			return m, m.startRecording()
		}
	}
	return m, nil
}

func (m *Model) startRecording() tea.Cmd {
	if err := m.audio.Start(); err != nil {
		slog.Error("dictator: mic failed", "err", err)
		m.state = stateError
		m.errMsg = "mic failed: " + err.Error()
		return nil
	}
	m.state = stateRecording
	return tick()
}

// pause stops arecord and flushes any in-progress utterance to the queue.
func (m *Model) pause() tea.Cmd {
	pcm := m.audio.Stop()
	m.recBytes += len(pcm)
	m.seg.Feed(pcm)
	m.state = statePaused
	if seg := m.seg.Flush(); seg != nil {
		return m.enqueue(seg)
	}
	return nil
}

// shutdown flushes pending audio and quits once queued transcriptions finish.
func (m *Model) shutdown() tea.Cmd {
	var cmd tea.Cmd
	if m.state == stateRecording {
		cmd = m.pause()
	}
	if m.busy || cmd != nil {
		m.state = stateFinishing
		return cmd
	}
	return tea.Quit
}

// enqueue hands a segment to the transcriber, serializing so segments land in
// the file in spoken order.
func (m *Model) enqueue(seg []byte) tea.Cmd {
	if m.busy {
		m.queue = append(m.queue, seg)
		return tick()
	}
	return tea.Batch(m.transcribe(seg), tick())
}

func (m *Model) transcribe(seg []byte) tea.Cmd {
	m.busy = true
	stt := m.stt
	return func() tea.Msg {
		text, err := stt.Transcribe(seg, sampleRate)
		return resultMsg{text: text, err: err}
	}
}

// writeLine appends the utterance to the output file (synced, so a battery
// pull loses at most the in-flight segment) and to the on-screen tail.
func (m *Model) writeLine(text string) {
	if _, err := m.file.WriteString(text + "\n"); err != nil {
		slog.Error("dictator: write failed", "err", err)
		m.appendLine("[write error: " + err.Error() + "]")
		return
	}
	m.file.Sync()
	m.appendLine(text)
}

func (m *Model) appendLine(text string) {
	m.lines = append(m.lines, text)
	if len(m.lines) > 200 {
		m.lines = m.lines[len(m.lines)-200:]
	}
}

func (m *Model) elapsed() time.Duration {
	return (time.Duration(m.recBytes) * time.Second / bytesPerSecond).Round(time.Second)
}
