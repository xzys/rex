package dictator

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func (m *Model) View() string {
	width := m.width
	if width <= 0 {
		width = 60
	}
	height := m.height
	if height <= 0 {
		height = 20
	}

	header := m.headerLine(width)
	footer := m.footerLine(width)

	// Transcript tail fills the space between header and footer.
	bodyH := height - 2
	if bodyH < 1 {
		bodyH = 1
	}
	body := m.transcriptTail(width, bodyH)

	return header + "\n" + body + "\n" + footer
}

func (m *Model) headerLine(width int) string {
	var status string
	switch m.state {
	case stateInitializing:
		status = yellow.Render(fmt.Sprintf("INITIALIZING %s", spinnerFrames[m.frame%len(spinnerFrames)]))
	case stateRecording:
		status = red.Render("● REC ") + m.elapsed().String()
	case statePaused:
		status = dim.Render("‖ PAUSED ") + m.elapsed().String()
	case stateFinishing:
		status = yellow.Render(fmt.Sprintf("FINISHING %s", spinnerFrames[m.frame%len(spinnerFrames)]))
	case stateError:
		status = red.Render("ERROR: " + m.errMsg)
	}
	if n := len(m.queue) + boolToInt(m.busy); n > 0 && m.state != stateInitializing {
		status += yellow.Render(fmt.Sprintf("  %s STT %d", spinnerFrames[m.frame%len(spinnerFrames)], n))
	}
	return padRight(status, width)
}

func (m *Model) footerLine(width int) string {
	left := dim.Render("A PAUSE/RESUME  B EXIT")
	right := dim.Render(m.path)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// transcriptTail renders the last lines of the transcript, wrapped to width,
// bottom-aligned so new text appears just above the footer.
func (m *Model) transcriptTail(width, height int) string {
	var wrapped []string
	for _, line := range m.lines {
		wrapped = append(wrapped, wrapLine(line, width)...)
	}
	if len(wrapped) > height {
		wrapped = wrapped[len(wrapped)-height:]
	}
	for len(wrapped) < height {
		wrapped = append([]string{""}, wrapped...)
	}
	return strings.Join(wrapped, "\n")
}

func wrapLine(s string, width int) []string {
	if width < 1 {
		width = 1
	}
	var out []string
	var cur string
	for _, word := range strings.Fields(s) {
		switch {
		case cur == "":
			cur = word
		case len(cur)+1+len(word) <= width:
			cur += " " + word
		default:
			out = append(out, cur)
			cur = word
		}
	}
	if cur != "" || len(out) == 0 {
		out = append(out, cur)
	}
	return out
}

func padRight(s string, width int) string {
	if gap := width - lipgloss.Width(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
