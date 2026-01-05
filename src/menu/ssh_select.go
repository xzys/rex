package menu

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func GetSSHHosts(configHosts []string) ([]string, error) {
	if len(configHosts) > 0 {
		return configHosts, nil
	}
	return parseSSHConfig()
}

func parseSSHConfig() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".ssh", "config")
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hosts []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "host ") {
			host := strings.TrimSpace(strings.TrimPrefix(line, line[:5]))
			if host != "*" && !strings.Contains(host, "*") {
				hosts = append(hosts, host)
			}
		}
	}

	return hosts, scanner.Err()
}

type SSHSelect struct {
	hosts    []string
	cursor   int
	selected string
	width    int
	height   int
}

func NewSSHSelect(hosts []string) *SSHSelect {
	return &SSHSelect{
		hosts:  hosts,
		cursor: 0,
	}
}

func (s *SSHSelect) Run() (string, error) {
	p := tea.NewProgram(s, tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		return "", err
	}
	return m.(*SSHSelect).selected, nil
}

func (s *SSHSelect) Init() tea.Cmd {
	return nil
}

func (s *SSHSelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return s, tea.Quit
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(s.hosts)-1 {
				s.cursor++
			}
		case "enter":
			s.selected = s.hosts[s.cursor]
			return s, tea.Quit
		}
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	}
	return s, nil
}

func (s *SSHSelect) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("6")).
		MarginBottom(1)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		PaddingLeft(2)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("6")).
		PaddingLeft(2).
		PaddingRight(1)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Select SSH Host"))
	b.WriteString("\n\n")

	for i, host := range s.hosts {
		if i == s.cursor {
			b.WriteString(selectedStyle.Render("▸ " + host))
		} else {
			b.WriteString(normalStyle.Render("  " + host))
		}
		b.WriteString("\n")
	}

	content := lipgloss.NewStyle().
		Padding(2, 4).
		Render(b.String())

	barStyle := lipgloss.NewStyle().
		Width(s.width).
		Foreground(lipgloss.Color("8")).
		Padding(0, 1)

	bar := barStyle.Render("↑/↓ navigate • enter select • esc cancel")

	contentHeight := strings.Count(content, "\n")
	spacerHeight := s.height - contentHeight - 1
	if spacerHeight < 0 {
		spacerHeight = 0
	}

	spacer := lipgloss.NewStyle().Height(spacerHeight).Render("")

	return lipgloss.JoinVertical(lipgloss.Left, content, spacer, bar)
}
