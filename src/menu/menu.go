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
	"github.com/xrfr/t-rex/v2/server"
)

type Menu struct {
	config         cfg.Config
	cursor         int
	width          int
	height         int
	itemsPerRow    int
	rowsFit        int
	rowOffset      int
	selected       bool
	sshHost        *string
	confirmInstall *cfg.App // when non-nil, render an install-prompt modal
	runInstall     *cfg.App // set when the user confirms; consumed by Run()
	inMoreView     bool     // when true, show the uninstalled-apps sub-menu
}

// moreSentinel is the Command used for the "more" tile on the main menu.
const moreSentinel = "__more__"

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
	if m.runInstall != nil {
		return m.runInstallScript(m.runInstall.Name)
	}
	if m.selected {
		app := m.SelectedApp()

		slog.Info("opening app", "name", app.Name, "command", app.Command)

		// keyboard operator uses window name to identify the app and change keybindings
		cmd := exec.Command("tmux", "rename-window", app.Name)
		if err := cmd.Run(); err != nil {
			slog.Warn("failed to rename window", "name", app.Name, "err", err)
		}

		// Apps flagged `voice: true` (e.g. codex, pi) auto-open the keyboard pane
		// in voice mode. Done via server so keyboardActive state stays in sync.
		if app.Voice {
			if err := server.SendOpenKeyboard(true); err != nil {
				slog.Warn("failed to auto-open voice keyboard", "name", app.Name, "err", err)
			}
		}

		if app.Command == "__tmux__" && m.sshHost != nil {
			return m.runSSHTmux()
		}

		if app.Command == "__new_window__" {
			return exec.Command("tmux", "new-window", "-c", "#{pane_current_path}").Run()
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

	script := `tmux resize-pane -Z && tmux attach`

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

// runInstallScript extracts the embedded install script for `name` and execs
// bash with it, replacing the menu process. The script runs in the menu's
// tmux window so the user can see progress; when it exits, the window closes.
func (m *Menu) runInstallScript(name string) error {
	path, err := writeInstallScript(name)
	if err != nil {
		return err
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		return fmt.Errorf("bash not found: %w", err)
	}
	return syscall.Exec(bash, []string{bash, path}, os.Environ())
}

func (m *Menu) effectiveApps() []cfg.App {
	// "more" sub-menu: only uninstalled config apps that ship an install script.
	if m.inMoreView {
		var more []cfg.App
		for _, a := range m.config.Apps {
			if isAppInstalled(a) {
				continue
			}
			if _, ok := installScript(a.Name); !ok {
				continue
			}
			more = append(more, a)
		}
		return more
	}

	var apps []cfg.App
	if m.sshHost != nil {
		tmuxApp := cfg.App{Name: "tmux", Icon: "▪", Color: "3", Command: "__tmux__"}
		apps = append([]cfg.App{tmuxApp}, m.config.Apps...)
	} else {
		shellApp := cfg.App{Name: "shell", Icon: "$", Color: "8", Command: "__new_window__"}
		apps = append([]cfg.App{shellApp}, m.config.Apps...)
	}
	// SSH mode: can't check what's installed on the remote, so show everything as-is.
	if m.sshHost != nil {
		return apps
	}
	// Local mode: only show installed apps on the main menu; uninstalled apps
	// live behind the "more" tile.
	installed := make([]cfg.App, 0, len(apps))
	hasMore := false
	for _, a := range apps {
		if isAppInstalled(a) {
			installed = append(installed, a)
			continue
		}
		if _, ok := installScript(a.Name); ok {
			hasMore = true
		}
	}
	if hasMore {
		installed = append(installed, cfg.App{Name: "more", Icon: "+", Color: "6", Command: moreSentinel})
	}
	return installed
}

// firstMissingIdx returns the index of the first uninstalled app, or -1 when
// no row break is needed. Used in SSH mode (where we can't check installed
// state) and elsewhere now that installed/missing live in separate views.
func (m *Menu) firstMissingIdx(apps []cfg.App) int {
	if m.sshHost != nil || m.inMoreView {
		return -1
	}
	for i, a := range apps {
		if !isAppInstalled(a) {
			return i
		}
	}
	return -1
}

func (m *Menu) Init() tea.Cmd {
	return nil
}

func (m *Menu) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Install-confirm modal intercepts input until dismissed.
		if m.confirmInstall != nil {
			switch msg.String() {
			case "enter":
				m.runInstall = m.confirmInstall
				m.confirmInstall = nil
				return m, tea.Quit
			case "esc", "b", "q":
				m.confirmInstall = nil
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "b":
			if m.inMoreView {
				m.inMoreView = false
				m.cursor = 0
				m.rowOffset = 0
				return m, nil
			}
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
			apps := m.effectiveApps()
			if m.cursor < len(apps) {
				app := apps[m.cursor]
				if app.Command == moreSentinel {
					m.inMoreView = true
					m.cursor = 0
					m.rowOffset = 0
					return m, nil
				}
				if !isAppInstalled(app) {
					if _, ok := installScript(app.Name); ok {
						m.confirmInstall = &app
						return m, nil
					}
					// no install script — fall through and try to launch; the
					// exec.LookPath in Run() will surface the failure.
				}
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
	gridArea := m.height - (layout.MarginY * 2)
	if m.sshHost != nil {
		gridArea -= 1
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
	missingStart := m.firstMissingIdx(apps)

	for i, app := range apps {
		// Force a row break before the first uninstalled app so missing tiles
		// start on a fresh row below the installed ones.
		if missingStart >= 0 && i == missingStart && len(currentRow) > 0 {
			rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, currentRow...))
			currentRow = nil
		}

		style := baseStyle.
			BorderForeground(lipgloss.Color(app.Color)).
			Foreground(lipgloss.Color(app.Color))

		isLastInRow := len(currentRow) == m.itemsPerRow-1 || i == len(apps)-1
		if !isLastInRow {
			style = style.MarginRight(layout.Gap)
		}

		missing := !isAppInstalled(app)
		if missing {
			style = style.
				BorderForeground(lipgloss.Color("8")).
				Foreground(lipgloss.Color("8"))
		}

		if i == m.cursor {
			bg := lipgloss.Color(app.Color)
			if missing {
				bg = lipgloss.Color("8")
			}
			style = style.
				Background(bg).
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

	grid := lipgloss.JoinVertical(lipgloss.Left, gappedRows...)

	gridHeight := m.height
	if m.sshHost != nil {
		gridHeight = m.height - 1
	}
	innerW := m.width - 2*layout.MarginX
	innerH := gridHeight - 2*layout.MarginY
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	placed := lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, grid)
	if m.confirmInstall != nil {
		placed = lipgloss.Place(innerW, innerH, lipgloss.Center, lipgloss.Center, m.renderConfirmModal())
	}
	centered := lipgloss.NewStyle().Padding(layout.MarginY, layout.MarginX).Render(placed)

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

func (m *Menu) renderConfirmModal() string {
	app := m.confirmInstall
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(app.Color)).Render(app.Name + " is not installed")
	body := lipgloss.NewStyle().Foreground(lipgloss.Color("7")).Render("install it now?")
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("A: install   B: cancel")
	content := lipgloss.JoinVertical(lipgloss.Center, title, "", body, "", hint)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(app.Color)).
		Padding(1, 3).
		Render(content)
}

func (m *Menu) SelectedApp() *cfg.App {
	apps := m.effectiveApps()
	if m.cursor >= 0 && m.cursor < len(apps) {
		return &apps[m.cursor]
	}
	return nil
}
