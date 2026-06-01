package keyboard

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xrfr/t-rex/v2/keyrepeat"
	"github.com/xrfr/t-rex/v2/protocol"
)

// DPadHoldTickMsg fires while a dpad direction is held in simple mode so the
// cursor keeps moving in that direction without the user re-tapping. Gen
// guards against stale ticks from a prior press: tea.Tick can't be cancelled,
// so a tick scheduled for an earlier press would otherwise fire later and
// nudge the cursor an extra step (the "skipped letters" bug).
type DPadHoldTickMsg struct {
	Key int
	Gen int
}

type KeyboardModelSimple struct {
	layout      [][]string
	selectedRow int
	selectedCol int
	rowWidths   []int
	heldDPad    int // -1 when nothing held
	holdGen     int // bumped on every dpad down/up to invalidate stale ticks
	layers      layerState
	pendingCmd  tea.Cmd
}

func NewKeyboardModelSimple(layout [][]string) *KeyboardModelSimple {
	rowWidths := make([]int, len(layout))
	for i, row := range layout {
		rowWidths[i] = len(row)
	}
	return &KeyboardModelSimple{
		layout:      layout,
		selectedRow: 0,
		selectedCol: 0,
		rowWidths:   rowWidths,
		heldDPad:    -1,
	}
}

// DrainCmd returns and clears any pending tea.Cmd (used for dpad hold ticks).
func (m *KeyboardModelSimple) DrainCmd() tea.Cmd {
	cmd := m.pendingCmd
	m.pendingCmd = nil
	return cmd
}

// HandleMsg processes hold ticks queued by HandleKey.
func (m *KeyboardModelSimple) HandleMsg(msg tea.Msg) {
	if t, ok := msg.(DPadHoldTickMsg); ok {
		if t.Gen == m.holdGen && m.heldDPad == t.Key && isDPad(t.Key) {
			m.moveDPad(t.Key)
			m.pendingCmd = dpadHoldTick(t.Key, m.holdGen, keyrepeat.Interval)
		}
	}
}

func dpadHoldTick(key, gen int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return DPadHoldTickMsg{Key: key, Gen: gen}
	})
}

func isDPad(key int) bool {
	switch key {
	case protocol.KeyDPadUp, protocol.KeyDPadDown, protocol.KeyDPadLeft, protocol.KeyDPadRight:
		return true
	}
	return false
}

func (m *KeyboardModelSimple) moveDPad(key int) {
	switch key {
	case protocol.KeyDPadUp:
		if m.selectedRow > 0 {
			m.selectedRow--
		} else {
			m.selectedRow = len(m.rowWidths) - 1
		}
		if m.selectedCol >= m.rowWidths[m.selectedRow] {
			m.selectedCol = m.rowWidths[m.selectedRow] - 1
		}
	case protocol.KeyDPadDown:
		if m.selectedRow < len(m.rowWidths)-1 {
			m.selectedRow++
		} else {
			m.selectedRow = 0
		}
		if m.selectedCol >= m.rowWidths[m.selectedRow] {
			m.selectedCol = m.rowWidths[m.selectedRow] - 1
		}
	case protocol.KeyDPadLeft:
		if m.selectedCol > 0 {
			m.selectedCol--
		} else {
			m.selectedCol = m.rowWidths[m.selectedRow] - 1
		}
	case protocol.KeyDPadRight:
		if m.selectedCol < m.rowWidths[m.selectedRow]-1 {
			m.selectedCol++
		} else {
			m.selectedCol = 0
		}
	}
}

func (m *KeyboardModelSimple) HandleKey(key int, isKeyDown bool) *string {
	if isDPad(key) {
		if isKeyDown {
			m.moveDPad(key)
			m.heldDPad = key
			m.holdGen++
			m.pendingCmd = dpadHoldTick(key, m.holdGen, keyrepeat.Delay)
		} else if m.heldDPad == key {
			m.heldDPad = -1
			m.holdGen++ // invalidate any in-flight repeat tick for this hold
		}
		return nil
	}
	if !isKeyDown {
		return nil
	}
	if key == protocol.KeyA {
		if m.selectedRow >= 0 && m.selectedRow < len(m.layout) &&
			m.selectedCol >= 0 && m.selectedCol < len(m.layout[m.selectedRow]) {
			return m.layers.resolve(m.layout[m.selectedRow][m.selectedCol])
		}
	}
	return nil
}

func (m KeyboardModelSimple) RenderKey(i, j int, key string) (string, string, string) {
	glyph := m.layers.glyph(key)
	if i == m.selectedRow && j == m.selectedCol {
		return "2", "2", glyph
	}
	// Highlight the active layer toggles so the current mode is visible.
	if m.layers.active(key) {
		return "3", "3", glyph
	}
	return "7", "0", glyph
}
