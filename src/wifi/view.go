package wifi

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	connectedSt   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	favoriteSt    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	dimSt         = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	cursorSt      = lipgloss.NewStyle().Background(lipgloss.Color("237"))
	statusSt      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errSt         = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("rex wifi") + "  ")
	b.WriteString(dimSt.Render("(connman)"))
	b.WriteString("\n\n")

	if len(m.services) == 0 {
		b.WriteString(dimSt.Render("no networks. press r to scan."))
	} else {
		for i, s := range m.services {
			row := formatRow(s)
			if i == m.cursor {
				row = cursorSt.Render(row)
			}
			b.WriteString(row)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	help := "↑/↓ select  enter connect  d disconnect  x forget  r scan  q quit"
	b.WriteString(dimSt.Render(help))
	b.WriteString("\n")

	if m.status != "" {
		st := statusSt
		if strings.HasPrefix(m.status, "err:") {
			st = errSt
		}
		b.WriteString(st.Render(m.status))
	}

	return b.String()
}

func formatRow(s Service) string {
	flag := flagsFor(s)
	name := s.Name
	if name == "" {
		name = "<hidden>"
	}
	sec := "open"
	if len(s.Security) > 0 {
		sec = strings.Join(s.Security, ",")
	}
	bar := signalBar(s.Strength)

	row := fmt.Sprintf(" %-3s %-24s %-12s %s %3d%%", flag, truncate(name, 24), sec, bar, s.Strength)

	switch {
	case isConnected(s.State):
		return connectedSt.Render(row)
	case s.Favorite:
		return favoriteSt.Render(row)
	default:
		return row
	}
}

func flagsFor(s Service) string {
	var f strings.Builder
	if isConnected(s.State) {
		f.WriteString("*")
	} else {
		f.WriteString(" ")
	}
	if s.AutoConnect {
		f.WriteString("A")
	} else {
		f.WriteString(" ")
	}
	if s.Favorite {
		f.WriteString("F")
	} else {
		f.WriteString(" ")
	}
	return f.String()
}

func signalBar(strength uint8) string {
	const bars = "▁▂▃▄▅▆▇█"
	idx := int(strength) * (len(bars) / 4) / 100
	if idx < 0 {
		idx = 0
	}
	if idx > 3 {
		idx = 3
	}
	// 4-segment bar, fill up to idx
	out := []rune("    ")
	r := []rune(bars)
	for i := 0; i <= idx; i++ {
		out[i] = r[i*2]
	}
	return string(out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
