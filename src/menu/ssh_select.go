package menu

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SSHSelect struct {
	hosts    []SSHHost
	cursor   int
	selected string
	width    int
	height   int
}

func NewSSHSelect(hosts []SSHHost) *SSHSelect {
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
	exec.Command("tmux", "rename-window", "menu").Run()
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
			s.selected = s.hosts[s.cursor].Name
			return s, tea.Quit
		}
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
	}
	return s, nil
}

func (s *SSHSelect) View() string {
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("4")).
		Padding(0, 1)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("7")).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("4")).
		Padding(0, 1)

	var b strings.Builder

	var tableContent strings.Builder
	if len(s.hosts) == 0 {
		tableContent.WriteString("No SSH hosts found")
	} else {
		colWidths := s.calculateColumnWidths()

		header := s.formatCell("Name", colWidths[0]) +
			s.formatCell("User", colWidths[1]) +
			s.formatCell("Host", colWidths[2]) +
			s.formatCell("Port", colWidths[3]) +
			s.formatCell("Identity", colWidths[4])
		tableContent.WriteString(headerStyle.Render(header))
		tableContent.WriteString("\n")

		totalWidth := colWidths[0] + colWidths[1] + colWidths[2] + colWidths[3] + colWidths[4]
		separatorWidth := totalWidth
		if separatorWidth < 1 {
			separatorWidth = 1
		}
		tableContent.WriteString(strings.Repeat("─", separatorWidth))
		tableContent.WriteString("\n")

		totalWidth = colWidths[0] + colWidths[1] + colWidths[2] + colWidths[3] + colWidths[4]
		emptyRow := strings.Repeat(" ", totalWidth)

		contentPadding := 4
		borderLines := 2
		borderPadding := 2
		headerLines := 2
		controlsLines := 2
		safetyMargin := 1
		overhead := contentPadding + borderLines + borderPadding + headerLines + controlsLines + safetyMargin

		availableRows := s.height - overhead
		if availableRows < 1 {
			availableRows = 1
		}

		for i := 0; i < availableRows; i++ {
			if i < len(s.hosts) {
				host := s.hosts[i]
				name := host.Name
				if name == "" {
					name = "-"
				}
				user := host.User
				if user == "" {
					user = "-"
				}
				hostAddr := host.Host
				if hostAddr == "" {
					hostAddr = "-"
				}
				port := host.Port
				if port == "" {
					port = "22"
				}
				identityFile := host.IdentityFile
				if identityFile == "" {
					identityFile = "-"
				}

				row := s.formatCell(name, colWidths[0]) +
					s.formatCell(user, colWidths[1]) +
					s.formatCell(hostAddr, colWidths[2]) +
					s.formatCell(port, colWidths[3]) +
					s.formatCell(identityFile, colWidths[4])

				if i == s.cursor {
					tableContent.WriteString(selectedStyle.Render(row))
				} else {
					tableContent.WriteString(normalStyle.Render(row))
				}
			} else {
				tableContent.WriteString(normalStyle.Render(emptyRow))
			}
			tableContent.WriteString("\n")
		}

		barStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Padding(0, 1)

		barText := "↑/↓ navigate • enter select • esc cancel"
		tableContent.WriteString("\n")
		tableContent.WriteString(barStyle.Render(barText))
	}

	tableStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2)

	table := tableStyle.Render(tableContent.String())
	b.WriteString(table)

	content := lipgloss.NewStyle().
		Padding(2, 4).
		Render(b.String())

	return content
}

func (s *SSHSelect) calculateColumnWidths() [5]int {
	contentPadding := 4
	borderPadding := 2
	availableWidth := s.width - (contentPadding * 2) - (borderPadding * 2) - 2

	if availableWidth < 20 {
		availableWidth = 20
	}

	percentages := [5]float64{0.20, 0.15, 0.25, 0.08, 0.32}
	colWidths := [5]int{}

	for i, pct := range percentages {
		colWidths[i] = int(float64(availableWidth) * pct)
	}

	minWidths := [5]int{4, 4, 4, 4, 8}
	for i := range colWidths {
		if colWidths[i] < minWidths[i] {
			colWidths[i] = minWidths[i]
		}
	}

	totalWidth := 0
	for _, w := range colWidths {
		totalWidth += w
	}

	diff := availableWidth - totalWidth
	if diff != 0 {
		colWidths[4] += diff
		if colWidths[4] < minWidths[4] {
			colWidths[4] = minWidths[4]
		}
	}

	return colWidths
}

func (s *SSHSelect) formatCell(value string, width int) string {
	if width < 1 {
		width = 1
	}
	if len(value) > width-2 && width > 5 {
		value = value[:width-5] + "..."
	} else if len(value) > width {
		value = value[:width]
	}
	padding := width - len(value)
	if padding < 0 {
		padding = 0
	}
	return value + strings.Repeat(" ", padding)
}
