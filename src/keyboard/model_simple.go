package keyboard

import (
	"github.com/xrfr/t-rex/v2/protocol"
)

type KeyboardModelSimple struct {
	layout      [][]string
	selectedRow int
	selectedCol int
	rowWidths   []int
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
	}
}

func (m *KeyboardModelSimple) HandleKey(key int, isKeyDown bool) *string {
	if !isKeyDown {
		return nil
	}
	switch key {
	case protocol.KeyDPadUp:
		if m.selectedRow > 0 {
			m.selectedRow--
			if m.selectedCol >= m.rowWidths[m.selectedRow] {
				m.selectedCol = m.rowWidths[m.selectedRow] - 1
			}
		}
	case protocol.KeyDPadDown:
		if m.selectedRow < len(m.rowWidths)-1 {
			m.selectedRow++
			if m.selectedCol >= m.rowWidths[m.selectedRow] {
				m.selectedCol = m.rowWidths[m.selectedRow] - 1
			}
		}
	case protocol.KeyDPadLeft:
		if m.selectedCol > 0 {
			m.selectedCol--
		}
	case protocol.KeyDPadRight:
		if m.selectedCol < m.rowWidths[m.selectedRow]-1 {
			m.selectedCol++
		}
	case protocol.KeyA:
		// Return the currently selected key
		if m.selectedRow >= 0 && m.selectedRow < len(m.layout) &&
			m.selectedCol >= 0 && m.selectedCol < len(m.layout[m.selectedRow]) {
			key := m.layout[m.selectedRow][m.selectedCol]
			if mapped, ok := specialKeyMap[key]; ok {
				mappedString := string(mapped)
				return &mappedString
			}
			return &key
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
