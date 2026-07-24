package wifi

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	sectionStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	rowStyle         = lipgloss.NewStyle()
	selectedRowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("0")).Background(lipgloss.Color("4"))
	connectedSt      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	favoriteSt       = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	dimSt            = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	statusSt         = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	errSt            = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	viewWidth := m.width
	if viewWidth < 40 {
		viewWidth = 40
	}
	viewHeight := m.height
	if viewHeight < 8 {
		viewHeight = 8
	}

	borderLines := 2
	horizontalPadding := 4 // Padding(1, 2)

	innerWidth := viewWidth - borderLines - horizontalPadding
	if innerWidth < 20 {
		innerWidth = 20
	}

	help := dimSt.Render(truncate("↑/↓ • A conn • X disc • Y refr • R3 forget • B back", innerWidth))
	if m.pwTarget.Path != "" {
		help = dimSt.Render(truncate("type passphrase • A connect • tab show/hide • B cancel", innerWidth))
	}

	var top strings.Builder
	top.WriteString(titleStyle.Render("Wifi"))
	top.WriteString("\n\n")
	if m.pwTarget.Path != "" {
		top.WriteString(m.renderPasswordPrompt(innerWidth))
		return finalizeBox(top.String(), help, viewWidth, viewHeight)
	}
	top.WriteString(sectionStyle.Render("Networks"))
	top.WriteString("\n")

	if len(m.services) == 0 {
		top.WriteString(dimSt.Render("no networks. press Y to scan."))
	} else {
		for i, s := range m.services {
			row := formatRow(s, innerWidth, false)
			if i == m.cursor {
				row = selectedRowStyle.Render(row)
			} else {
				row = rowStyle.Render(row)
			}
			top.WriteString(row)
			if i < len(m.services)-1 {
				top.WriteString("\n")
			}
		}
	}

	if len(m.saved) > 0 {
		top.WriteString("\n\n")
		top.WriteString(sectionStyle.Render("Saved"))
		top.WriteString("\n")
		base := len(m.services)
		for i, s := range m.saved {
			row := formatRow(s, innerWidth, true)
			if base+i == m.cursor {
				row = selectedRowStyle.Render(row)
			} else {
				row = dimSt.Render(row)
			}
			top.WriteString(row)
			if i < len(m.saved)-1 {
				top.WriteString("\n")
			}
		}
	}

	if m.status != "" {
		st := statusSt
		if strings.HasPrefix(m.status, "err:") {
			st = errSt
		}
		top.WriteString("\n\n")
		top.WriteString(st.Render(m.status))
	}

	return finalizeBox(top.String(), help, viewWidth, viewHeight)
}

func finalizeBox(topContent, help string, viewWidth, viewHeight int) string {
	borderLines := 2
	verticalPadding := 2
	topLines := strings.Count(topContent, "\n") + 1
	helpLines := 1
	spacerLines := viewHeight - borderLines - verticalPadding - topLines - helpLines
	if spacerLines < 1 {
		spacerLines = 1
	}

	var content strings.Builder
	content.WriteString(topContent)
	content.WriteString("\n")
	for i := 0; i < spacerLines; i++ {
		content.WriteString("\n")
	}
	content.WriteString(help)

	// Pin the box to the full pane width so it doesn't collapse to the widest
	// content line when there are few/no networks (WiFi off). lipgloss Width
	// covers content+padding but not the border, so subtract the 2 border
	// columns to land at exactly viewWidth.
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("4")).
		Padding(1, 2).
		Width(viewWidth - borderLines).
		Render(content.String())
}

func (m *Model) renderPasswordPrompt(width int) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("Passphrase"))
	b.WriteString("\n")
	b.WriteString(dimSt.Render(truncate("connecting to "+m.pwTarget.Name, width)))
	b.WriteString("\n\n")

	display := m.pwBuf
	if !m.pwShow {
		display = strings.Repeat("•", len([]rune(m.pwBuf)))
	}
	if display == "" {
		display = dimSt.Render("(enter passphrase)")
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1).
		Render(display + "▏")
	b.WriteString(box)

	if m.status != "" {
		st := statusSt
		if strings.HasPrefix(m.status, "err:") {
			st = errSt
		}
		b.WriteString("\n\n")
		b.WriteString(st.Render(m.status))
	}
	return b.String()
}

// formatRow renders a service row. `savedOnly` rows have no strength/security
// info (not currently visible), and skip the connected/favorite inner styling
// so the caller's dim style applies cleanly.
func formatRow(s Service, width int, savedOnly bool) string {
	if width < 20 {
		width = 20
	}

	flag := flagsFor(s)
	name := s.Name
	if name == "" {
		name = "<hidden>"
	}

	var right string
	if savedOnly {
		right = "saved"
	} else {
		sec := "open"
		if len(s.Security) > 0 {
			sec = strings.Join(s.Security, ",")
		}
		bar := signalBar(s.Strength)
		right = fmt.Sprintf("%-12s %s %3d%%", truncate(sec, 12), bar, s.Strength)
	}
	left := fmt.Sprintf(" %-3s %s", flag, name)

	maxLeft := width - len(right) - 1
	if maxLeft < 8 {
		maxLeft = 8
	}
	if lipgloss.Width(left) > maxLeft {
		nameMax := maxLeft - 5
		if nameMax < 1 {
			nameMax = 1
		}
		left = fmt.Sprintf(" %-3s %s", flag, truncate(name, nameMax))
	}

	spaces := width - lipgloss.Width(left) - lipgloss.Width(right)
	if spaces < 1 {
		spaces = 1
	}
	row := left + strings.Repeat(" ", spaces) + right

	if savedOnly {
		return row
	}
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
	out := []rune("    ")
	r := []rune(bars)
	for i := 0; i <= idx; i++ {
		out[i] = r[i*2]
	}
	return string(out)
}

// truncate cuts `s` so its rendered terminal width is ≤ n. n is the visible
// cell budget (not byte length), so multibyte glyphs like ↑/•/⏎ count as one.
// If trimming is needed, the last visible cell is replaced with "…".
func truncate(s string, n int) string {
	if lipgloss.Width(s) <= n {
		return s
	}
	if n <= 1 {
		runes := []rune(s)
		if len(runes) == 0 {
			return ""
		}
		return string(runes[:1])
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > n-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	b.WriteRune('…')
	return b.String()
}
