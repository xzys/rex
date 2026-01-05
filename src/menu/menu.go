package menu

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

type App struct {
	Name      string `yaml:"name"`
	Icon      string `yaml:"icon"`
	Color     string `yaml:"color"`
	Command   string `yaml:"command"`
	Exclusive bool   `yaml:"exclusive"`
}

type Layout struct {
	Gap        int `yaml:"gap"`
	MarginX    int `yaml:"margin_x"`
	MarginY    int `yaml:"margin_y"`
	SquareSize int `yaml:"square_size"`
}

type Config struct {
	Layout Layout `yaml:"layout"`
	Apps   []App  `yaml:"apps"`
}

type Menu struct {
	config      Config
	cursor      int
	width       int
	height      int
	itemsPerRow int
	selected    bool
	sshHost     *string
}

// findConfigPath tries multiple paths and returns the first one that exists
func findConfigPath(userPath string) (string, error) {
	if userPath != "" {
		if _, err := os.Stat(userPath); err == nil {
			return userPath, nil
		} else {
			return "", err
		}
	}

	if homeDir, err := os.UserHomeDir(); err == nil {
		defaultPath := filepath.Join(homeDir, ".config", "rex", "config.yaml")
		if _, err := os.Stat(defaultPath); err == nil {
			return defaultPath, nil
		}
	}

	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml", nil
	}

	return "", os.ErrNotExist
}

func LoadConfig(path string) (Config, error) {
	configPath, err := findConfigPath(path)
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return Config{}, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}

func NewMenu(configPath string, sshHost *string) (*Menu, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	return &Menu{
		config:      config,
		cursor:      0,
		itemsPerRow: 4,
		sshHost:     sshHost,
	}, nil
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
	cmd := exec.Command("ssh", "-t", *m.sshHost, "sh", "-ic", commandStr)
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
	itemWidth := layout.SquareSize + layout.Gap
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

func (m *Menu) SelectedApp() *App {
	if m.cursor >= 0 && m.cursor < len(m.config.Apps) {
		return &m.config.Apps[m.cursor]
	}
	return nil
}
