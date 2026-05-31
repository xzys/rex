package wifi

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestViewFillsWidth guards the regression where the wifi box collapsed to the
// widest content line (leaving a right margin) when there were no networks to
// pad it to full width — i.e. WiFi off / empty list. The box must always span
// the full pane width, keep its top and bottom borders, and never exceed the
// pane height, regardless of how many networks are present.
func TestViewFillsWidth(t *testing.T) {
	connected := Service{
		Name: "Sai Sravya 04G", State: "ready", Strength: 65,
		Security: []string{"psk", "wps"}, Favorite: true, AutoConnect: true,
	}
	cases := map[string][]Service{
		"empty":         {},
		"one_connected": {connected},
	}
	sizes := [][2]int{{73, 23}, {64, 20}, {100, 30}, {40, 12}}

	for name, svcs := range cases {
		for _, sz := range sizes {
			w, h := sz[0], sz[1]
			m := &Model{services: svcs, width: w, height: h}
			lines := strings.Split(m.View(), "\n")

			if len(lines) > h {
				t.Errorf("%s @ %dx%d: %d lines exceeds pane height", name, w, h, len(lines))
			}
			for i, ln := range lines {
				if lw := lipgloss.Width(ln); lw != w {
					t.Errorf("%s @ %dx%d: line %d width = %d, want %d (%q)", name, w, h, i, lw, w, ln)
					break
				}
			}
			if !isBorder(lines[0]) {
				t.Errorf("%s @ %dx%d: missing top border: %q", name, w, h, lines[0])
			}
			if !isBorder(lines[len(lines)-1]) {
				t.Errorf("%s @ %dx%d: missing bottom border: %q", name, w, h, lines[len(lines)-1])
			}
		}
	}
}

func isBorder(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "┌") || strings.HasPrefix(t, "└") ||
		strings.HasPrefix(t, "╭") || strings.HasPrefix(t, "╰")
}
