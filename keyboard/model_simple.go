package keyboard

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

func (m *KeyboardModelSimple) HandleKey(key int) error {
	switch key {
	case keyDPadUp:
		if m.selectedRow > 0 {
			m.selectedRow--
			if m.selectedCol >= m.rowWidths[m.selectedRow] {
				m.selectedCol = m.rowWidths[m.selectedRow] - 1
			}
		}
	case keyDPadDown:
		if m.selectedRow < len(m.rowWidths)-1 {
			m.selectedRow++
			if m.selectedCol >= m.rowWidths[m.selectedRow] {
				m.selectedCol = m.rowWidths[m.selectedRow] - 1
			}
		}
	case keyDPadLeft:
		if m.selectedCol > 0 {
			m.selectedCol--
		}
	case keyDPadRight:
		if m.selectedCol < m.rowWidths[m.selectedRow]-1 {
			m.selectedCol++
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
