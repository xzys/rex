package menu

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	cfg "github.com/xrfr/t-rex/v2/config"
)

type Menu struct {
	config      cfg.Config
	cursor      int
	width       int
	height      int
	itemsPerRow int
	selected    bool
	sshHost     *string
}

func NewMenu(sshHost *string) (*Menu, error) {
	config, err := cfg.LoadConfig()
	if err != nil {
		return nil, err
	}
	return NewMenuWithConfig(config, sshHost), nil
}

func NewMenuWithConfig(config cfg.Config, sshHost *string) *Menu {
	return &Menu{
		config:      config,
		cursor:      0,
		itemsPerRow: 4,
		sshHost:     sshHost,
	}
}

func (m *Menu) Run() error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	if m.selected {
		app := m.SelectedApp()

		slog.Info("opening app", "name", app.Name, "command", app.Command)

		// keyboard operator uses window name to identify the app and change keybindings
		cmd := exec.Command("tmux", "rename-window", app.Name)
		if err := cmd.Run(); err != nil {
			slog.Warn("failed to rename window", "name", app.Name, "err", err)
		}

		if m.sshHost != nil {
			return m.runSSH(app.Command)
		}

		fields := strings.Fields(app.Command)
		if len(fields) == 0 {
			return fmt.Errorf("empty command")
		}

		execPath, err := exec.LookPath(fields[0])
		if err != nil {
			return err
		}

		args := append([]string{execPath}, fields[1:]...)

		if app.Exclusive {
			return m.runExclusive(args)
		}
		return syscall.Exec(execPath, args, os.Environ())
	}
	return nil
}

func (m *Menu) runSSH(command string) error {
	commandStr := `'` + command + `'`
	cmd := exec.Command("ssh", "-v", "-o", "ConnectTimeout=10", "-t", *m.sshHost, "sh", "-ic", commandStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func (m *Menu) runExclusive(args []string) error {
	rexPath, _ := os.Executable()
	rexArgs := append([]string{rexPath, "exclusive"}, args...)
	return syscall.Exec(rexPath, rexArgs, os.Environ())
}

func (m *Menu) Init() tea.Cmd {
	return nil
}

func (m *Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "left", "h":
			if m.cursor > 0 {
				m.cursor--
			}
		case "right", "l":
			if m.cursor < len(m.config.Apps)-1 {
				m.cursor++
			}
		case "up", "k":
			if m.cursor >= m.itemsPerRow {
				m.cursor -= m.itemsPerRow
			}
		case "down", "j":
			if m.cursor+m.itemsPerRow < len(m.config.Apps) {
				m.cursor += m.itemsPerRow
			}
		case "enter":
			if m.cursor < len(m.config.Apps) {
				m.selected = true
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.calculateItemsPerRow()
	}

	return m, nil
}

func (m *Menu) calculateItemsPerRow() {
	layout := m.config.Layout
	availableWidth := m.width - (layout.MarginX * 2)
	borderWidth := 2 // left + right border chars
	itemWidth := layout.SquareSize + borderWidth + layout.Gap
	if itemWidth > 0 {
		m.itemsPerRow = availableWidth / itemWidth
		if m.itemsPerRow < 1 {
			m.itemsPerRow = 1
		}
	}
}

func (m *Menu) View() string {
	layout := m.config.Layout

	baseStyle := lipgloss.NewStyle().
		Width(layout.SquareSize).
		Height(layout.SquareSize/2).
		Align(lipgloss.Center, lipgloss.Center).
		Border(lipgloss.NormalBorder())

	if len(m.config.Apps) == 0 {
		msg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Render("no apps configured\n~/.config/rex/config.yaml")
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(msg)
	}

	var rows []string
	var currentRow []string

	for i, app := range m.config.Apps {
		style := baseStyle.
			BorderForeground(lipgloss.Color(app.Color)).
			Foreground(lipgloss.Color(app.Color)).
			MarginRight(layout.Gap)

		if i == m.cursor {
			style = style.
				Background(lipgloss.Color(app.Color)).
				Foreground(lipgloss.Color("#000000"))
		}

		content := app.Icon + "\n" + app.Name
		square := style.Render(content)
		currentRow = append(currentRow, square)

		if len(currentRow) >= m.itemsPerRow {
			row := lipgloss.JoinHorizontal(lipgloss.Top, currentRow...)
			rows = append(rows, row)
			currentRow = nil
		}
	}

	if len(currentRow) > 0 {
		row := lipgloss.JoinHorizontal(lipgloss.Top, currentRow...)
		rows = append(rows, row)
	}

	gapStyle := lipgloss.NewStyle().MarginBottom(layout.Gap / 2)
	var gappedRows []string
	for _, row := range rows {
		gappedRows = append(gappedRows, gapStyle.Render(row))
	}

	grid := lipgloss.JoinVertical(lipgloss.Left, gappedRows...)

	content := lipgloss.NewStyle().
		Padding(layout.MarginY, layout.MarginX).
		Render(grid)

	if m.sshHost != nil {
		barStyle := lipgloss.NewStyle().
			Width(m.width).
			Height(1).
			Background(lipgloss.Color("4")).
			Foreground(lipgloss.Color("0")).
			Align(lipgloss.Left, lipgloss.Center).
			Padding(0, 1)

		barText := "open remotely in " + *m.sshHost
		bar := barStyle.Render(barText)

		contentHeight := strings.Count(content, "\n")
		spacerHeight := m.height - contentHeight - 1
		if spacerHeight < 0 {
			spacerHeight = 0
		}

		spacer := lipgloss.NewStyle().Height(spacerHeight).Render("")
		combined := lipgloss.JoinVertical(lipgloss.Left, content, spacer, bar)

		containerStyle := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height)

		return containerStyle.Render(combined)
	}

	return content
}

func (m *Menu) SelectedApp() *cfg.App {
	if m.cursor >= 0 && m.cursor < len(m.config.Apps) {
		return &m.config.Apps[m.cursor]
	}
	return nil
}
