package keyboard

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/xzys/TR3X/v2/protocol"
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
	client         *Client // Deprecated - kept for compatibility but not used
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
	triggers := []int{protocol.KeyDPadUp, protocol.KeyDPadDown, protocol.KeyDPadLeft, protocol.KeyDPadRight, protocol.KeyX, protocol.KeyA, protocol.KeyY, protocol.KeyB}
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

	return &KeyboardModelChorded{
		triggers:       triggerNames,
		layoutNumbers:  layoutNumbers,
		chordMap:       chordMap,
		chordAltMap:    chordAltMap,
		chordActiveMap: chordActiveMap,
		client:         nil, // Client no longer needed in model
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
	chordMap[encodeChord(modifierNone, protocol.KeyDPadLeft)] = 'a'
	chordMap[encodeChord(modifierNone, protocol.KeyDPadUp)] = 's'
	chordMap[encodeChord(modifierNone, protocol.KeyDPadDown)] = 'd'
	chordMap[encodeChord(modifierNone, protocol.KeyDPadRight)] = 'f'
	chordMap[encodeChord(modifierNone, protocol.KeyX)] = 'h'
	chordMap[encodeChord(modifierNone, protocol.KeyA)] = 'j'
	chordMap[encodeChord(modifierNone, protocol.KeyY)] = 'k'
	chordMap[encodeChord(modifierNone, protocol.KeyB)] = 'l'

	// L1 modifier
	chordMap[encodeChord(modifierL1, protocol.KeyDPadLeft)] = 'q'
	chordMap[encodeChord(modifierL1, protocol.KeyDPadUp)] = 'w'
	chordMap[encodeChord(modifierL1, protocol.KeyDPadDown)] = 'e'
	chordMap[encodeChord(modifierL1, protocol.KeyDPadRight)] = 'r'
	chordMap[encodeChord(modifierL1, protocol.KeyX)] = 'u'
	chordMap[encodeChord(modifierL1, protocol.KeyA)] = 'i'
	chordMap[encodeChord(modifierL1, protocol.KeyY)] = 'o'
	chordMap[encodeChord(modifierL1, protocol.KeyB)] = 'p'

	// R1 modifier
	chordMap[encodeChord(modifierR1, protocol.KeyDPadLeft)] = 'z'
	chordMap[encodeChord(modifierR1, protocol.KeyDPadUp)] = 'x'
	chordMap[encodeChord(modifierR1, protocol.KeyDPadDown)] = 'c'
	chordMap[encodeChord(modifierR1, protocol.KeyDPadRight)] = 'v'
	chordMap[encodeChord(modifierR1, protocol.KeyX)] = 'g'
	chordMap[encodeChord(modifierR1, protocol.KeyA)] = 'b'
	chordMap[encodeChord(modifierR1, protocol.KeyY)] = 'n'
	chordMap[encodeChord(modifierR1, protocol.KeyB)] = 'm'

	// L2 modifier
	chordMap[encodeChord(modifierL2, protocol.KeyDPadLeft)] = specialKeyShift
	chordMap[encodeChord(modifierL2, protocol.KeyDPadUp)] = 't'
	chordMap[encodeChord(modifierL2, protocol.KeyDPadDown)] = 'y'
	chordMap[encodeChord(modifierL2, protocol.KeyDPadRight)] = specialKeySpace
	chordMap[encodeChord(modifierL2, protocol.KeyX)] = specialKeyDelete
	chordMap[encodeChord(modifierL2, protocol.KeyA)] = specialKeyEnter
	chordMap[encodeChord(modifierL2, protocol.KeyY)] = specialKeyNumbers
	chordMap[encodeChord(modifierL2, protocol.KeyB)] = specialKeyClose

	chordAltMap := make(map[int]rune)
	// Base triggers (no modifier) - numbers
	chordAltMap[encodeChord(modifierNone, protocol.KeyDPadLeft)] = '1'
	chordAltMap[encodeChord(modifierNone, protocol.KeyDPadUp)] = '2'
	chordAltMap[encodeChord(modifierNone, protocol.KeyDPadDown)] = '3'
	chordAltMap[encodeChord(modifierNone, protocol.KeyDPadRight)] = '4'
	chordAltMap[encodeChord(modifierNone, protocol.KeyX)] = '5'
	chordAltMap[encodeChord(modifierNone, protocol.KeyA)] = '6'
	chordAltMap[encodeChord(modifierNone, protocol.KeyY)] = '7'
	chordAltMap[encodeChord(modifierNone, protocol.KeyB)] = '8'

	// L1 modifier - spaces
	chordAltMap[encodeChord(modifierL1, protocol.KeyDPadLeft)] = ' '
	chordAltMap[encodeChord(modifierL1, protocol.KeyDPadUp)] = ' '
	chordAltMap[encodeChord(modifierL1, protocol.KeyDPadDown)] = ' '
	chordAltMap[encodeChord(modifierL1, protocol.KeyDPadRight)] = ' '
	chordAltMap[encodeChord(modifierL1, protocol.KeyX)] = ' '
	chordAltMap[encodeChord(modifierL1, protocol.KeyA)] = ' '
	chordAltMap[encodeChord(modifierL1, protocol.KeyY)] = ' '
	chordAltMap[encodeChord(modifierL1, protocol.KeyB)] = ' '

	// R1 modifier - spaces
	chordAltMap[encodeChord(modifierR1, protocol.KeyDPadLeft)] = ' '
	chordAltMap[encodeChord(modifierR1, protocol.KeyDPadUp)] = ' '
	chordAltMap[encodeChord(modifierR1, protocol.KeyDPadDown)] = ' '
	chordAltMap[encodeChord(modifierR1, protocol.KeyDPadRight)] = ' '
	chordAltMap[encodeChord(modifierR1, protocol.KeyX)] = ' '
	chordAltMap[encodeChord(modifierR1, protocol.KeyA)] = ' '
	chordAltMap[encodeChord(modifierR1, protocol.KeyY)] = ' '
	chordAltMap[encodeChord(modifierR1, protocol.KeyB)] = ' '

	// L2 modifier - spaces
	chordAltMap[encodeChord(modifierL2, protocol.KeyDPadLeft)] = ' '
	chordAltMap[encodeChord(modifierL2, protocol.KeyDPadUp)] = ' '
	chordAltMap[encodeChord(modifierL2, protocol.KeyDPadDown)] = ' '
	chordAltMap[encodeChord(modifierL2, protocol.KeyDPadRight)] = ' '
	chordAltMap[encodeChord(modifierL2, protocol.KeyX)] = ' '
	chordAltMap[encodeChord(modifierL2, protocol.KeyA)] = ' '
	chordAltMap[encodeChord(modifierL2, protocol.KeyY)] = ' '
	chordAltMap[encodeChord(modifierL2, protocol.KeyB)] = ' '
	return chordMap, chordAltMap
}

func (m *KeyboardModelChorded) resetState() {
	// Reset modifiers after trigger
	m.modifier = modifierNone
}

func (m *KeyboardModelChorded) HandleKey(key int) (*string, error) {
	prevModifier := m.modifier
	switch key {
	// Modifier keys - toggle state
	case protocol.KeyL1, protocol.KeyR1, protocol.KeyL2, protocol.KeyR2:
		switch key {
		case protocol.KeyL1:
			m.modifier = modifierL1
		case protocol.KeyR1:
			m.modifier = modifierR1
		case protocol.KeyL2:
			m.modifier = modifierL2
		case protocol.KeyR2:
			m.modifier = modifierR2
		}
		if prevModifier == m.modifier {
			m.resetState()
		}
		return nil, nil
	// Trigger keys - complete the chord
	case protocol.KeyDPadUp, protocol.KeyDPadDown, protocol.KeyDPadLeft, protocol.KeyDPadRight, protocol.KeyA, protocol.KeyB, protocol.KeyX, protocol.KeyY:
		keyStr := m.executeChord(key)
		m.resetState()
		return &keyStr, nil
	}
	return nil, nil
}

func (m *KeyboardModelChorded) executeChord(triggerKey int) string {
	// Determine which key to press from chord map
	chordKey := encodeChord(m.modifier, triggerKey)
	char, exists := m.chordMap[chordKey]
	if !exists {
		slog.Error("chord not found in map", "modifier", m.modifier, "trigger", triggerKey)
		return ""
	}

	// Update local state based on chord
	if char == specialKeyShift {
		m.modeShift = !m.modeShift
	} else if char == specialKeyNumbers {
		m.modeNumbers = !m.modeNumbers
	}

	// Return the key string to press
	return string(char)
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
