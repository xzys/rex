package keyboard

import (
	"github.com/xrfr/t-rex/v2/protocol"
)

type KeyboardModelSimple struct {
	selectedRow int
	selectedCol int
	rowWidths   []int
}

func NewKeyboardModelSimple() *KeyboardModelSimple {
	rowWidths := make([]int, len(keyboardLayout))
	for i, row := range keyboardLayout {
		rowWidths[i] = len(row)
	}
	return &KeyboardModelSimple{
		selectedRow: 0,
		selectedCol: 0,
		rowWidths:   rowWidths,
	}
}

func (m *KeyboardModelSimple) HandleKey(key int) *string {
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
		if m.selectedRow >= 0 && m.selectedRow < len(keyboardLayout) &&
			m.selectedCol >= 0 && m.selectedCol < len(keyboardLayout[m.selectedRow]) {
			key := keyboardLayout[m.selectedRow][m.selectedCol]
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
	return "0", "0", key
}
