package settings

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cfg "github.com/xrfr/t-rex/v2/config"
)

type Model struct {
	config        cfg.Config
	cursor        int
	width         int
	height        int
	kbModes       []string
	kbModeIndex   int
	kbLayouts     []string
	kbLayoutIndex int
	kbViews       []string
	kbViewIndex   int
	sttModes      []string
	sttModeIndex  int
	defaultVoice  bool
	status        string
}

const (
	rowKeyboardMode = iota
	rowKeyboardLayout
	rowKeyboardView
	rowDefaultVoice
	rowSTTMode
	rowCount
)

func Run() error {
	config, err := cfg.LoadConfig()
	if err != nil {
		return err
	}

	m := NewModel(config)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func NewModel(config cfg.Config) *Model {
	kbModes := cfg.KeyboardModes()
	kbModeIndex := indexOf(kbModes, config.KeyboardMode)

	kbLayouts := cfg.KeyboardLayouts()
	kbLayoutIndex := indexOf(kbLayouts, cfg.NormalizeKeyboardLayout(config.KeyboardLayout))

	kbViews := cfg.KeyboardViews()
	kbViewIndex := indexOf(kbViews, cfg.NormalizeKeyboardView(config.KeyboardView))

	sttModes := cfg.STTModes()
	sttModeIndex := indexOf(sttModes, cfg.NormalizeSTTMode(config.STTMode))

	return &Model{
		config:        config,
		kbModes:       kbModes,
		kbModeIndex:   kbModeIndex,
		kbLayouts:     kbLayouts,
		kbLayoutIndex: kbLayoutIndex,
		kbViews:       kbViews,
		kbViewIndex:   kbViewIndex,
		sttModes:      sttModes,
		sttModeIndex:  sttModeIndex,
		defaultVoice:  config.DefaultVoice,
	}
}

func indexOf(values []string, target string) int {
	for i, v := range values {
		if v == target {
			return i
		}
	}
	return 0
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc", "b":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < rowCount-1 {
				m.cursor++
			}
		case "left", "h":
			m.cycleMode(-1)
		case "right", "l", "enter", "a":
			m.cycleMode(1)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m *Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	rowStyle := lipgloss.NewStyle()
	selectedRowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("4"))
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	viewWidth := m.width
	if viewWidth < 40 {
		viewWidth = 40
	}
	viewHeight := m.height
	if viewHeight < 8 {
		viewHeight = 8
	}

	borderLines := 2
	horizontalPadding := 4 // Padding(1, 2)
	verticalPadding := 2   // Padding(1, 2)

	innerWidth := viewWidth - borderLines - horizontalPadding
	if innerWidth < 20 {
		innerWidth = 20
	}

	defaultVoiceStr := "off"
	if m.defaultVoice {
		defaultVoiceStr = "on"
	}

	rows := []struct {
		label string
		value string
	}{
		{"Chording mode", m.kbModes[m.kbModeIndex]},
		{"Keyboard layout", m.kbLayouts[m.kbLayoutIndex]},
		{"Keyboard view", m.kbViews[m.kbViewIndex]},
		{"Keyboard Default Voice Mode", defaultVoiceStr},
		{"STT mode", m.sttModes[m.sttModeIndex]},
	}

	help := hintStyle.Render("↑/↓ select row • ←/→ change • esc back")

	var top strings.Builder
	top.WriteString(titleStyle.Render("Settings"))
	top.WriteString("\n\n")
	for i, r := range rows {
		line := m.formatRow(r.label, r.value, innerWidth)
		if m.cursor == i {
			line = selectedRowStyle.Render(line)
		} else {
			line = rowStyle.Render(line)
		}
		if i > 0 {
			top.WriteString("\n")
		}
		top.WriteString(line)
	}
	if m.status != "" {
		top.WriteString("\n\n")
		top.WriteString(statusStyle.Render(m.status))
	}
	topContent := top.String()
	topLines := strings.Count(topContent, "\n") + 1

	helpLines := 1
	spacerLines := viewHeight - borderLines - verticalPadding - topLines - helpLines
	if spacerLines < 1 {
		spacerLines = 1
	}

	var content strings.Builder
	content.WriteString(topContent)
	content.WriteString("\n")
	for i := 0; i < spacerLines; i++ {
		content.WriteString("\n")
	}
	content.WriteString(help)

	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2).
		Render(content.String())

	return box
}

func (m *Model) cycleMode(direction int) {
	switch m.cursor {
	case rowKeyboardMode:
		if n := len(m.kbModes); n > 0 {
			m.kbModeIndex = (m.kbModeIndex + direction + n) % n
			m.config.KeyboardMode = m.kbModes[m.kbModeIndex]
		}
	case rowKeyboardLayout:
		if n := len(m.kbLayouts); n > 0 {
			m.kbLayoutIndex = (m.kbLayoutIndex + direction + n) % n
			m.config.KeyboardLayout = m.kbLayouts[m.kbLayoutIndex]
		}
	case rowKeyboardView:
		if n := len(m.kbViews); n > 0 {
			m.kbViewIndex = (m.kbViewIndex + direction + n) % n
			m.config.KeyboardView = m.kbViews[m.kbViewIndex]
		}
	case rowSTTMode:
		if n := len(m.sttModes); n > 0 {
			m.sttModeIndex = (m.sttModeIndex + direction + n) % n
			m.config.STTMode = m.sttModes[m.sttModeIndex]
		}
	case rowDefaultVoice:
		m.defaultVoice = !m.defaultVoice
		m.config.DefaultVoice = m.defaultVoice
	default:
		return
	}
	if err := cfg.SaveConfig(m.config); err != nil {
		m.status = fmt.Sprintf("save failed: %v", err)
		return
	}
	m.status = "saved"
}

func (m *Model) formatRow(name, value string, width int) string {
	if width < 10 {
		width = 10
	}

	if len(name) > width-6 {
		name = name[:width-6]
	}
	if len(value) > width-6 {
		value = value[:width-6]
	}

	spaces := width - len(name) - len(value)
	if spaces < 1 {
		spaces = 1
	}
	return name + strings.Repeat(" ", spaces) + value
}
