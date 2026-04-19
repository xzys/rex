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
	rowsFit     int
	rowOffset   int
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

		if app.Command == "__tmux__" && m.sshHost != nil {
			return m.runSSHTmux()
		}

		if app.Command == "__new_window__" {
			return exec.Command("tmux", "new-window").Run()
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

const sshControlSocket = "/tmp/rex-ssh-ctl"
const sshInfoFile = "/tmp/rex-ssh-info"

func (m *Menu) runSSHTmux() error {
	// Write socket path + host so SSHTmuxOperator can fire commands via ControlMaster
	info := sshControlSocket + " " + *m.sshHost
	if err := os.WriteFile(sshInfoFile, []byte(info), 0600); err != nil {
		return fmt.Errorf("failed to write ssh info: %w", err)
	}
	defer os.Remove(sshInfoFile)

	script := `tmux set status off` +
		` && tmux resize-pane -Z` +
		` && tmux attach` +
		` ; tmux set status on`

	cmd := exec.Command("ssh",
		"-M", "-S", sshControlSocket,
		"-o", "ConnectTimeout=10",
		"-t", *m.sshHost,
		script,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
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

func (m *Menu) effectiveApps() []cfg.App {
	if m.sshHost != nil {
		tmuxApp := cfg.App{Name: "tmux", Icon: "▪", Color: "3", Command: "__tmux__"}
		return append([]cfg.App{tmuxApp}, m.config.Apps...)
	}
	shellApp := cfg.App{Name: "shell", Icon: "$", Color: "8", Command: "__new_window__"}
	return append([]cfg.App{shellApp}, m.config.Apps...)
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
			m.scrollToCursor()
		case "right", "l":
			if m.cursor < len(m.effectiveApps())-1 {
				m.cursor++
			}
			m.scrollToCursor()
		case "up", "k":
			if m.cursor >= m.itemsPerRow {
				m.cursor -= m.itemsPerRow
			}
			m.scrollToCursor()
		case "down", "j":
			if m.cursor+m.itemsPerRow < len(m.effectiveApps()) {
				m.cursor += m.itemsPerRow
			}
			m.scrollToCursor()
		case "enter":
			if m.cursor < len(m.effectiveApps()) {
				m.selected = true
				return m, tea.Quit
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.calculateLayout()
		m.scrollToCursor()
	}

	return m, nil
}

func (m *Menu) calculateLayout() {
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

	cardHeight := layout.SquareSize/2 + 2 // content + top/bottom border
	rowHeight := cardHeight + layout.Gap/2
	gridArea := m.height
	if m.sshHost != nil {
		gridArea = m.height - 1
	}
	if rowHeight > 0 {
		m.rowsFit = gridArea / rowHeight
		if m.rowsFit < 1 {
			m.rowsFit = 1
		}
	}
}

func (m *Menu) scrollToCursor() {
	if m.itemsPerRow <= 0 || m.rowsFit <= 0 {
		return
	}
	cursorRow := m.cursor / m.itemsPerRow
	if cursorRow < m.rowOffset {
		m.rowOffset = cursorRow
	}
	if cursorRow >= m.rowOffset+m.rowsFit {
		m.rowOffset = cursorRow - m.rowsFit + 1
	}
	if m.rowOffset < 0 {
		m.rowOffset = 0
	}
}

func (m *Menu) View() string {
	layout := m.config.Layout

	baseStyle := lipgloss.NewStyle().
		Width(layout.SquareSize).
		Height(layout.SquareSize/2).
		Align(lipgloss.Center, lipgloss.Center).
		Border(lipgloss.NormalBorder())

	apps := m.effectiveApps()

	if len(apps) == 0 {
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

	for i, app := range apps {
		style := baseStyle.
			BorderForeground(lipgloss.Color(app.Color)).
			Foreground(lipgloss.Color(app.Color))

		isLastInRow := len(currentRow) == m.itemsPerRow-1 || i == len(apps)-1
		if !isLastInRow {
			style = style.MarginRight(layout.Gap)
		}

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

	if m.rowOffset > len(rows)-m.rowsFit {
		m.rowOffset = len(rows) - m.rowsFit
	}
	if m.rowOffset < 0 {
		m.rowOffset = 0
	}
	end := m.rowOffset + m.rowsFit
	if end > len(rows) {
		end = len(rows)
	}
	visibleRows := rows[m.rowOffset:end]

	gapStyle := lipgloss.NewStyle().MarginBottom(layout.Gap / 2)
	var gappedRows []string
	for _, row := range visibleRows {
		gappedRows = append(gappedRows, gapStyle.Render(row))
	}

	grid := lipgloss.JoinVertical(lipgloss.Center, gappedRows...)

	gridHeight := m.height
	if m.sshHost != nil {
		gridHeight = m.height - 1
	}
	centered := lipgloss.Place(m.width, gridHeight, lipgloss.Center, lipgloss.Center, grid)

	if m.sshHost != nil {
		barStyle := lipgloss.NewStyle().
			Width(m.width).
			Height(1).
			Background(lipgloss.Color("4")).
			Foreground(lipgloss.Color("0")).
			Align(lipgloss.Left, lipgloss.Center).
			Padding(0, 1)

		bar := barStyle.Render("open remotely in " + *m.sshHost)
		return lipgloss.JoinVertical(lipgloss.Left, centered, bar)
	}

	return centered
}

func (m *Menu) SelectedApp() *cfg.App {
	apps := m.effectiveApps()
	if m.cursor >= 0 && m.cursor < len(apps) {
		return &apps[m.cursor]
	}
	return nil
}
