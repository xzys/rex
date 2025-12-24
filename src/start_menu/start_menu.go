package start_menu

import (
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

type App struct {
	Name           string `yaml:"name"`
	Icon           string `yaml:"icon"`
	Color          string `yaml:"color"`
	ExecutablePath string `yaml:"executable_path"`
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

type StartMenu struct {
	config      Config
	cursor      int
	width       int
	height      int
	itemsPerRow int
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

func NewStartMenu(configPath string) (*StartMenu, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	return &StartMenu{
		config:      config,
		cursor:      0,
		itemsPerRow: 4,
	}, nil
}

func (s *StartMenu) Run() error {
	p := tea.NewProgram(s, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (s *StartMenu) Init() tea.Cmd {
	return nil
}

func (s *StartMenu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return s, tea.Quit
		case "left", "h":
			if s.cursor > 0 {
				s.cursor--
			}
		case "right", "l":
			if s.cursor < len(s.config.Apps)-1 {
				s.cursor++
			}
		case "up", "k":
			if s.cursor >= s.itemsPerRow {
				s.cursor -= s.itemsPerRow
			}
		case "down", "j":
			if s.cursor+s.itemsPerRow < len(s.config.Apps) {
				s.cursor += s.itemsPerRow
			}
		case "enter":
			if s.cursor < len(s.config.Apps) {
				// Return selected app - caller can handle execution
				return s, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		s.width = msg.Width
		s.height = msg.Height
		s.calculateItemsPerRow()
	}

	return s, nil
}

func (s *StartMenu) calculateItemsPerRow() {
	layout := s.config.Layout
	availableWidth := s.width - (layout.MarginX * 2)
	itemWidth := layout.SquareSize + layout.Gap
	if itemWidth > 0 {
		s.itemsPerRow = availableWidth / itemWidth
		if s.itemsPerRow < 1 {
			s.itemsPerRow = 1
		}
	}
}

func (s *StartMenu) View() string {
	layout := s.config.Layout

	baseStyle := lipgloss.NewStyle().
		Width(layout.SquareSize).
		Height(layout.SquareSize/2).
		Align(lipgloss.Center, lipgloss.Center).
		Border(lipgloss.NormalBorder())

	var rows []string
	var currentRow []string

	for i, app := range s.config.Apps {
		style := baseStyle.
			BorderForeground(lipgloss.Color(app.Color)).
			Foreground(lipgloss.Color(app.Color)).
			MarginRight(layout.Gap)

		if i == s.cursor {
			style = style.
				Background(lipgloss.Color(app.Color)).
				Foreground(lipgloss.Color("#000000"))
		}

		content := app.Icon + "\n" + app.Name
		square := style.Render(content)
		currentRow = append(currentRow, square)

		if len(currentRow) >= s.itemsPerRow {
			row := lipgloss.JoinHorizontal(lipgloss.Top, currentRow...)
			rows = append(rows, row)
			currentRow = nil
		}
	}

	// Handle remaining items in last row
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

	return lipgloss.NewStyle().
		Padding(layout.MarginY, layout.MarginX).
		Render(grid)
}

func (s *StartMenu) SelectedApp() *App {
	if s.cursor >= 0 && s.cursor < len(s.config.Apps) {
		return &s.config.Apps[s.cursor]
	}
	return nil
}
