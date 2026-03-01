package settings

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cfg "github.com/xrfr/t-rex/v2/config"
)

type Model struct {
	config    cfg.Config
	cursor    int
	width     int
	height    int
	modes     []string
	modeIndex int
	status    string
}

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
	modes := cfg.KeyboardModes()
	modeIndex := 0
	for i, mode := range modes {
		if mode == config.KeyboardMode {
			modeIndex = i
			break
		}
	}

	return &Model{
		config:    config,
		modes:     modes,
		modeIndex: modeIndex,
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		rowCount := 1
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

	row := m.formatRow("Chording mode", m.modes[m.modeIndex], innerWidth)
	if m.cursor == 0 {
		row = selectedRowStyle.Render(row)
	} else {
		row = rowStyle.Render(row)
	}

	help := hintStyle.Render("←/→ change • enter select • esc back")

	var top strings.Builder
	top.WriteString(titleStyle.Render("Settings"))
	top.WriteString("\n\n")
	top.WriteString(row)
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
	count := len(m.modes)
	if count == 0 {
		return
	}

	m.modeIndex = (m.modeIndex + direction + count) % count
	m.config.KeyboardMode = m.modes[m.modeIndex]
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
