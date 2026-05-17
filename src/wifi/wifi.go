package wifi

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/godbus/dbus/v5"
)

type tickMsg time.Time
type servicesMsg struct {
	visible []Service
	saved   []Service
}
type errMsg struct{ err error }
type doneMsg struct{ verb string }

type Model struct {
	client   *Client
	services []Service // visible from ConnMan
	saved    []Service // saved profiles not currently visible (Path empty)
	cursor   int       // index into services++saved
	width    int
	height   int
	status   string
	busy     string
	quitting bool

	// password entry: when pwTarget is non-zero we render a passphrase
	// prompt instead of the list. Submit (enter) calls ConnectWithPassphrase
	// and persists via Remember; esc cancels.
	pwTarget Service
	pwBuf    string
	pwShow   bool // mask off — show plaintext (for verification)
}

func Run() error {
	// Rename our tmux window so the wifi.js plugin engages (server keys the
	// active operator on window name). No-op when not running inside tmux.
	renameTmuxWindow("wifi")

	// Best-effort: mirror any SSID/key that EmulationStation has set in
	// batocera.conf into the persistent ConnMan provisioning file so it
	// auto-reconnects on next boot. Don't fail the TUI if this errors.
	_, _ = CaptureBatocera()

	c, err := NewClient()
	if err != nil {
		return err
	}
	defer c.Close()

	m := NewModel(c)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// NewModel constructs a wifi Model around an already-open ConnMan dbus
// client. The caller retains ownership of the client and is responsible for
// closing it. Useful when embedding the wifi screen inside another bubbletea
// program (e.g. settings) that wants to reuse the same connection.
func NewModel(c *Client) *Model {
	return &Model{client: c}
}

// List prints wifi services to stdout (non-interactive). Useful for scripts and smoke tests.
func List() error {
	_, _ = CaptureBatocera()

	c, err := NewClient()
	if err != nil {
		return err
	}
	defer c.Close()

	services, err := c.Wifi()
	if err != nil {
		return err
	}
	for _, s := range services {
		flag := flagsFor(s)
		sec := "open"
		if len(s.Security) > 0 {
			sec = s.Security[0]
		}
		name := s.Name
		if name == "" {
			name = "<hidden>"
		}
		fmt.Fprintf(os.Stdout, "%s %3d%%  %-8s  %s\n", flag, s.Strength, sec, name)
	}
	return nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.refresh(), tickEvery())
}

func tickEvery() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) refresh() tea.Cmd {
	return func() tea.Msg {
		s, err := m.client.Wifi()
		if err != nil {
			return errMsg{err}
		}
		return servicesMsg{visible: s, saved: savedOnly(s)}
	}
}

// savedOnly returns synthesized Services for profiles in ConfigPath that
// aren't represented in `visible`. Empty Path marks them as not-currently-
// reachable; consumers (View, key handlers) special-case those.
func savedOnly(visible []Service) []Service {
	profiles, err := LoadProfiles()
	if err != nil || len(profiles) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(visible))
	for _, s := range visible {
		if s.Name != "" {
			seen[s.Name] = true
		}
	}
	names := make([]string, 0, len(profiles))
	for ssid := range profiles {
		if !seen[ssid] {
			names = append(names, ssid)
		}
	}
	sort.Strings(names)
	out := make([]Service, 0, len(names))
	for _, ssid := range names {
		out = append(out, Service{Name: ssid, Favorite: true})
	}
	return out
}

func (m *Model) scan() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.ScanWifi(); err != nil {
			return errMsg{err}
		}
		return doneMsg{verb: "refresh"}
	}
}

func (m *Model) connect(path dbus.ObjectPath) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.Connect(path); err != nil {
			return errMsg{err}
		}
		return doneMsg{verb: "connect"}
	}
}

// connectWithPassphrase runs the agent-mediated Connect on the wifi
// service at `path` and, on success, persists the passphrase to
// ConfigPath so the network auto-reconnects on the next boot.
func (m *Model) connectWithPassphrase(s Service, passphrase string) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.ConnectWithPassphrase(s.Path, passphrase); err != nil {
			return errMsg{err}
		}
		// Best-effort persistence — failure here doesn't break the live
		// connection, but we want to surface it via the status line.
		if _, err := Remember(s.Name, passphrase); err != nil {
			return errMsg{fmt.Errorf("connected but persist failed: %w", err)}
		}
		return doneMsg{verb: "connect"}
	}
}

func (m *Model) disconnect(path dbus.ObjectPath) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.Disconnect(path); err != nil {
			return errMsg{err}
		}
		return doneMsg{verb: "disconnect"}
	}
}

func (m *Model) remove(s Service) tea.Cmd {
	return func() tea.Msg {
		// Always drop the persistent provisioning entry first so the network
		// doesn't come back on next boot. ConnMan.Remove fails for Immutable
		// services (Batocera-provisioned), which is fine — once we delete the
		// provisioning entry the service won't reappear.
		_, _ = Forget(s.Name)
		if s.Path != "" {
			if err := m.client.Remove(s.Path); err != nil && !isImmutableErr(err) {
				return errMsg{err}
			}
		}
		return doneMsg{verb: "forget"}
	}
}

func isImmutableErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "Not supported")
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tickMsg:
		return m, tea.Batch(m.refresh(), tickEvery())

	case servicesMsg:
		m.services = msg.visible
		m.saved = msg.saved
		if total := len(m.services) + len(m.saved); m.cursor >= total {
			m.cursor = max0(total - 1)
		}

	case errMsg:
		m.busy = ""
		m.status = "err: " + msg.err.Error()

	case doneMsg:
		m.busy = ""
		m.status = msg.verb + " ok"
		return m, m.refresh()

	case tea.KeyMsg:
		if m.busy != "" && msg.String() != "ctrl+c" {
			return m, nil
		}
		if m.pwTarget.Path != "" {
			return m.handlePasswordKey(msg)
		}
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.services)+len(m.saved)-1 {
				m.cursor++
			}
		case "r", "f5":
			m.busy = "refreshing"
			m.status = "refreshing…"
			return m, m.scan()
		case "enter":
			if s, ok := m.current(); ok && s.Path != "" {
				if needsPassphrase(s) {
					m.pwTarget = s
					m.pwBuf = ""
					m.pwShow = false
					m.status = ""
					return m, nil
				}
				m.busy = "connecting"
				m.status = "connecting to " + s.Name + "…"
				return m, m.connect(s.Path)
			}
		case "d", "f4":
			if s, ok := m.current(); ok && isConnected(s.State) {
				m.busy = "disconnecting"
				m.status = "disconnecting " + s.Name + "…"
				return m, m.disconnect(s.Path)
			}
		case "f6":
			if s, ok := m.current(); ok && s.Favorite {
				m.busy = "forgetting"
				m.status = "forgetting " + s.Name + "…"
				return m, m.remove(s)
			}
		}
	}

	return m, nil
}

// handlePasswordKey processes key events while the passphrase prompt is
// active. Most printable input arrives via the rex keyboard injecting
// `tmux send-keys -t 0 X`, so we need to accept the full visible-ASCII
// range, not just lowercase letters.
func (m *Model) handlePasswordKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.pwTarget = Service{}
		m.pwBuf = ""
		m.status = "cancelled"
		return m, nil
	case tea.KeyEnter:
		if m.pwBuf == "" {
			m.status = "passphrase required"
			return m, nil
		}
		s := m.pwTarget
		pw := m.pwBuf
		m.pwTarget = Service{}
		m.pwBuf = ""
		m.busy = "connecting"
		m.status = "connecting to " + s.Name + "…"
		return m, m.connectWithPassphrase(s, pw)
	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.pwBuf) > 0 {
			r := []rune(m.pwBuf)
			m.pwBuf = string(r[:len(r)-1])
		}
		return m, nil
	case tea.KeyTab:
		m.pwShow = !m.pwShow
		return m, nil
	case tea.KeySpace:
		m.pwBuf += " "
		return m, nil
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			if r >= 0x20 && r != 0x7f {
				m.pwBuf += string(r)
			}
		}
		return m, nil
	}
	return m, nil
}

// needsPassphrase reports whether attempting to Connect to `s` will
// require a fresh passphrase. Favorite services already have a stored
// passphrase (ConnMan or our provisioning file); open networks need
// nothing.
func needsPassphrase(s Service) bool {
	if s.Favorite {
		return false
	}
	for _, sec := range s.Security {
		switch sec {
		case "psk", "wpa", "wpa2", "wpa3", "ieee8021x":
			return true
		}
	}
	return false
}

func (m *Model) current() (Service, bool) {
	if m.cursor < 0 {
		return Service{}, false
	}
	if m.cursor < len(m.services) {
		return m.services[m.cursor], true
	}
	idx := m.cursor - len(m.services)
	if idx < len(m.saved) {
		return m.saved[idx], true
	}
	return Service{}, false
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

// renameTmuxWindow sets the current tmux window name to `name`. Best
// effort — silently no-ops outside tmux. Used so the server's per-window
// plugin loader picks up the wifi gamepad bindings even when the TUI is
// launched directly (not via the menu, which already renames the window).
func renameTmuxWindow(name string) {
	if os.Getenv("TMUX") == "" {
		return
	}
	_ = exec.Command("tmux", "rename-window", name).Run()
}
