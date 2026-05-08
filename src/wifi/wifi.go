package wifi

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/godbus/dbus/v5"
)

type tickMsg time.Time
type servicesMsg []Service
type errMsg struct{ err error }
type doneMsg struct{ verb string }

type Model struct {
	client   *Client
	services []Service
	cursor   int
	width    int
	height   int
	status   string
	busy     string
	quitting bool
}

func Run() error {
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
		return servicesMsg(s)
	}
}

func (m *Model) scan() tea.Cmd {
	return func() tea.Msg {
		if err := m.client.ScanWifi(); err != nil {
			return errMsg{err}
		}
		return doneMsg{verb: "scan"}
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

func (m *Model) disconnect(path dbus.ObjectPath) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.Disconnect(path); err != nil {
			return errMsg{err}
		}
		return doneMsg{verb: "disconnect"}
	}
}

func (m *Model) remove(path dbus.ObjectPath) tea.Cmd {
	return func() tea.Msg {
		if err := m.client.Remove(path); err != nil {
			return errMsg{err}
		}
		return doneMsg{verb: "forget"}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tickMsg:
		return m, tea.Batch(m.refresh(), tickEvery())

	case servicesMsg:
		m.services = []Service(msg)
		if m.cursor >= len(m.services) {
			m.cursor = max0(len(m.services) - 1)
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
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.services)-1 {
				m.cursor++
			}
		case "r":
			m.busy = "scanning"
			m.status = "scanning…"
			return m, m.scan()
		case "enter":
			if s, ok := m.current(); ok {
				m.busy = "connecting"
				m.status = "connecting to " + s.Name + "…"
				return m, m.connect(s.Path)
			}
		case "d":
			if s, ok := m.current(); ok && isConnected(s.State) {
				m.busy = "disconnecting"
				m.status = "disconnecting " + s.Name + "…"
				return m, m.disconnect(s.Path)
			}
		case "x":
			if s, ok := m.current(); ok && s.Favorite {
				m.busy = "forgetting"
				m.status = "forgetting " + s.Name + "…"
				return m, m.remove(s.Path)
			}
		}
	}

	return m, nil
}

func (m *Model) current() (Service, bool) {
	if m.cursor < 0 || m.cursor >= len(m.services) {
		return Service{}, false
	}
	return m.services[m.cursor], true
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
