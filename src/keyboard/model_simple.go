package keyboard

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xrfr/t-rex/v2/protocol"
)

const (
	dpadHoldInitialMs = 400
	dpadHoldRepeatMs  = 80
)

// DPadHoldTickMsg fires while a dpad direction is held in simple mode so the
// cursor keeps moving in that direction without the user re-tapping.
type DPadHoldTickMsg struct {
	Key int
}

type KeyboardModelSimple struct {
	layout      [][]string
	selectedRow int
	selectedCol int
	rowWidths   []int
	heldDPad    int // -1 when nothing held
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
		if m.heldDPad == t.Key && isDPad(t.Key) {
			m.moveDPad(t.Key)
			m.pendingCmd = dpadHoldTick(t.Key, dpadHoldRepeatMs)
		}
	}
}

func dpadHoldTick(key, ms int) tea.Cmd {
	return tea.Tick(time.Duration(ms)*time.Millisecond, func(time.Time) tea.Msg {
		return DPadHoldTickMsg{Key: key}
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
			m.pendingCmd = dpadHoldTick(key, dpadHoldInitialMs)
		} else if m.heldDPad == key {
			m.heldDPad = -1
		}
		return nil
	}
	if !isKeyDown {
		return nil
	}
	if key == protocol.KeyA {
		if m.selectedRow >= 0 && m.selectedRow < len(m.layout) &&
			m.selectedCol >= 0 && m.selectedCol < len(m.layout[m.selectedRow]) {
			k := m.layout[m.selectedRow][m.selectedCol]
			if mapped, ok := specialKeyMap[k]; ok {
				mappedString := string(mapped)
				return &mappedString
			}
			return &k
		}
	}
	return nil
}

func (m KeyboardModelSimple) RenderKey(i, j int, key string) (string, string, string) {
	if i == m.selectedRow && j == m.selectedCol {
		return "2", "2", key
	}
	return "7", "0", key
}
