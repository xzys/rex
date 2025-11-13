package keyboard

import (
	"fmt"
	"log/slog"
	"strings"
)

type KeyboardModelChorded struct {
	// Current modifier state
	modifier       int
	triggers       []string
	modeShift      bool
	modeNumbers    bool
	layoutNumbers  [][]rune
	chordMap       map[int]rune
	chordAltMap    map[int]rune
	chordActiveMap []map[string]string
	client         *Client
}

const (
	modifierNone = iota
	modifierL1
	modifierR1
	modifierL2
	modifierR2
)

// encodeChord encodes modifier (lower 4 bits) and trigger (upper 4 bits) into a single int
func encodeChord(modifier, trigger int) int {
	return modifier | (trigger << 4)
}

func NewKeyboardModelChorded(keyboardLayout [][]string) *KeyboardModelChorded {
	modifiers := []int{modifierNone, modifierL1, modifierR1, modifierL2, modifierR2}
	triggers := []int{keyDPadUp, keyDPadDown, keyDPadLeft, keyDPadRight, keyX, keyA, keyY, keyB}
	triggerNames := []string{"up", "down", "left", "right", "x", "a", "y", "b"}
	chordMap, chordAltMap := getChordMaps()
	chordActiveMap := make([]map[string]string, len(modifiers))

	layoutNumbers := make([][]rune, len(keyboardLayout))
	for i, row := range keyboardLayout {
		layoutNumbers[i] = make([]rune, len(row))
	}
	// static which keys are active for each modifier
	for mi, mod := range modifiers {
		chordActiveMap[mi] = make(map[string]string)
		for ti, trigger := range triggers {
			// for each mod/trigger combos, check all keys
			for i, row := range keyboardLayout {
				for j, key := range row {
					// encode the chord key
					chordMapKey := encodeChord(mod, trigger)
					chordKey := chordMap[chordMapKey]
					chordAltKey := chordAltMap[chordMapKey]

					if string(chordKey) == strings.ToLower(key) {
						// cache location of key in layout
						chordActiveMap[mi][fmt.Sprintf("%d,%d", i, j)] = triggerNames[ti]
						// make new layout of number keys
						layoutNumbers[i][j] = chordAltKey
					} else {
						if specialKey, ok := specialKeyMap[key]; ok && chordKey == specialKey {
							chordActiveMap[mi][fmt.Sprintf("%d,%d", i, j)] = triggerNames[ti]
						}
					}
				}
			}
		}
	}

	client, err := NewClient()
	if err != nil {
		slog.Error("failed to create client", "err", err)
		// Continue without client - will fail gracefully on executeChord
	}

	return &KeyboardModelChorded{
		triggers:       triggerNames,
		layoutNumbers:  layoutNumbers,
		chordMap:       chordMap,
		chordAltMap:    chordAltMap,
		chordActiveMap: chordActiveMap,
		client:         client,
	}
}

const (
	specialKeySpace   = ' '
	specialKeyShift   = 'S'
	specialKeyDelete  = 'D'
	specialKeyNumbers = '1'
	specialKeyClose   = 'X'
	specialKeyEnter   = 'E'
)

var specialKeyMap = map[string]rune{
	"SPC": specialKeySpace,
	"SFT": specialKeyShift,
	"DEL": specialKeyDelete,
	"123": specialKeyNumbers,
	"BCK": specialKeyClose,
	"ENT": specialKeyEnter,
}

func getChordMaps() (map[int]rune, map[int]rune) {
	chordMap := make(map[int]rune)
	// Base triggers (no modifier)
	chordMap[encodeChord(modifierNone, keyDPadLeft)] = 'a'
	chordMap[encodeChord(modifierNone, keyDPadUp)] = 's'
	chordMap[encodeChord(modifierNone, keyDPadDown)] = 'd'
	chordMap[encodeChord(modifierNone, keyDPadRight)] = 'f'
	chordMap[encodeChord(modifierNone, keyX)] = 'h'
	chordMap[encodeChord(modifierNone, keyA)] = 'j'
	chordMap[encodeChord(modifierNone, keyY)] = 'k'
	chordMap[encodeChord(modifierNone, keyB)] = 'l'

	// L1 modifier
	chordMap[encodeChord(modifierL1, keyDPadLeft)] = 'q'
	chordMap[encodeChord(modifierL1, keyDPadUp)] = 'w'
	chordMap[encodeChord(modifierL1, keyDPadDown)] = 'e'
	chordMap[encodeChord(modifierL1, keyDPadRight)] = 'r'
	chordMap[encodeChord(modifierL1, keyX)] = 'u'
	chordMap[encodeChord(modifierL1, keyA)] = 'i'
	chordMap[encodeChord(modifierL1, keyY)] = 'o'
	chordMap[encodeChord(modifierL1, keyB)] = 'p'

	// R1 modifier
	chordMap[encodeChord(modifierR1, keyDPadLeft)] = 'z'
	chordMap[encodeChord(modifierR1, keyDPadUp)] = 'x'
	chordMap[encodeChord(modifierR1, keyDPadDown)] = 'c'
	chordMap[encodeChord(modifierR1, keyDPadRight)] = 'v'
	chordMap[encodeChord(modifierR1, keyX)] = 'g'
	chordMap[encodeChord(modifierR1, keyA)] = 'b'
	chordMap[encodeChord(modifierR1, keyY)] = 'n'
	chordMap[encodeChord(modifierR1, keyB)] = 'm'

	// L2 modifier
	chordMap[encodeChord(modifierL2, keyDPadLeft)] = specialKeyShift
	chordMap[encodeChord(modifierL2, keyDPadUp)] = 't'
	chordMap[encodeChord(modifierL2, keyDPadDown)] = 'y'
	chordMap[encodeChord(modifierL2, keyDPadRight)] = specialKeySpace
	chordMap[encodeChord(modifierL2, keyX)] = specialKeyDelete
	chordMap[encodeChord(modifierL2, keyA)] = specialKeyEnter
	chordMap[encodeChord(modifierL2, keyY)] = specialKeyNumbers
	chordMap[encodeChord(modifierL2, keyB)] = specialKeyClose

	chordAltMap := make(map[int]rune)
	// Base triggers (no modifier) - numbers
	chordAltMap[encodeChord(modifierNone, keyDPadLeft)] = '1'
	chordAltMap[encodeChord(modifierNone, keyDPadUp)] = '2'
	chordAltMap[encodeChord(modifierNone, keyDPadDown)] = '3'
	chordAltMap[encodeChord(modifierNone, keyDPadRight)] = '4'
	chordAltMap[encodeChord(modifierNone, keyX)] = '5'
	chordAltMap[encodeChord(modifierNone, keyA)] = '6'
	chordAltMap[encodeChord(modifierNone, keyY)] = '7'
	chordAltMap[encodeChord(modifierNone, keyB)] = '8'

	// L1 modifier - spaces
	chordAltMap[encodeChord(modifierL1, keyDPadLeft)] = ' '
	chordAltMap[encodeChord(modifierL1, keyDPadUp)] = ' '
	chordAltMap[encodeChord(modifierL1, keyDPadDown)] = ' '
	chordAltMap[encodeChord(modifierL1, keyDPadRight)] = ' '
	chordAltMap[encodeChord(modifierL1, keyX)] = ' '
	chordAltMap[encodeChord(modifierL1, keyA)] = ' '
	chordAltMap[encodeChord(modifierL1, keyY)] = ' '
	chordAltMap[encodeChord(modifierL1, keyB)] = ' '

	// R1 modifier - spaces
	chordAltMap[encodeChord(modifierR1, keyDPadLeft)] = ' '
	chordAltMap[encodeChord(modifierR1, keyDPadUp)] = ' '
	chordAltMap[encodeChord(modifierR1, keyDPadDown)] = ' '
	chordAltMap[encodeChord(modifierR1, keyDPadRight)] = ' '
	chordAltMap[encodeChord(modifierR1, keyX)] = ' '
	chordAltMap[encodeChord(modifierR1, keyA)] = ' '
	chordAltMap[encodeChord(modifierR1, keyY)] = ' '
	chordAltMap[encodeChord(modifierR1, keyB)] = ' '

	// L2 modifier - spaces
	chordAltMap[encodeChord(modifierL2, keyDPadLeft)] = ' '
	chordAltMap[encodeChord(modifierL2, keyDPadUp)] = ' '
	chordAltMap[encodeChord(modifierL2, keyDPadDown)] = ' '
	chordAltMap[encodeChord(modifierL2, keyDPadRight)] = ' '
	chordAltMap[encodeChord(modifierL2, keyX)] = ' '
	chordAltMap[encodeChord(modifierL2, keyA)] = ' '
	chordAltMap[encodeChord(modifierL2, keyY)] = ' '
	chordAltMap[encodeChord(modifierL2, keyB)] = ' '
	return chordMap, chordAltMap
}

func (m *KeyboardModelChorded) resetState() {
	// Reset modifiers after trigger
	m.modifier = modifierNone
}

func (m *KeyboardModelChorded) HandleKey(key int) error {
	prevModifier := m.modifier
	switch key {
	// Modifier keys - toggle state
	case keyL1, keyR1, keyL2, keyR2:
		switch key {
		case keyL1:
			m.modifier = modifierL1
		case keyR1:
			m.modifier = modifierR1
		case keyL2:
			m.modifier = modifierL2
		case keyR2:
			m.modifier = modifierR2
		}
		if prevModifier == m.modifier {
			m.resetState()
		}
	// Trigger keys - complete the chord
	case keyDPadUp, keyDPadDown, keyDPadLeft, keyDPadRight, keyA, keyB, keyX, keyY:
		m.executeChord(key)
		m.resetState()
	}
	return nil
}

func (m *KeyboardModelChorded) executeChord(triggerKey int) {
	if m.client == nil {
		slog.Error("client not available")
		return
	}

	// Determine which key to press from chord map
	chordKey := encodeChord(m.modifier, triggerKey)
	char, exists := m.chordMap[chordKey]
	if !exists {
		slog.Error("chord not found in map", "modifier", m.modifier, "trigger", triggerKey)
		return
	}

	// Update local state based on chord
	if char == specialKeyShift {
		m.modeShift = !m.modeShift
	} else if char == specialKeyNumbers {
		m.modeNumbers = !m.modeNumbers
	}

	// Send key press request to server
	keyStr := string(char)
	if err := m.client.PressKey(keyStr); err != nil {
		slog.Error("failed to press key", "err", err, "key", keyStr)
		return
	}
}

func (m *KeyboardModelChorded) RenderKey(i, j int, key string) (string, string, string) {
	// Check if any modifiers are active
	if len(key) == 1 {
		if m.modeNumbers {
			key = string(m.layoutNumbers[i][j])
		} else if m.modeShift {
			key = strings.ToUpper(key)
		}
	}
	if trigger, exists := m.chordActiveMap[m.modifier][fmt.Sprintf("%d,%d", i, j)]; exists {
		switch trigger {
		case "up":
			return "7", "7", key
		case "down":
			return "7", "7", key
		case "left":
			return "7", "7", key
		case "right":
			return "7", "7", key
		case "x":
			return "3", "3", key
		case "y":
			return "2", "2", key
		case "b":
			return "1", "1", key
		case "a":
			return "4", "4", key
		}
	}
	return "0", "0", key
}

func (m *KeyboardModelChorded) Cleanup() {
	if m.client != nil {
		m.client.Close()
	}
}
