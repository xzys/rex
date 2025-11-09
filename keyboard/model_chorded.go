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
	chordMap       map[string]rune
	chordAltMap    map[string]rune
	chordActiveMap []map[string]string
}

const (
	modifierNone = iota
	modifierL1
	modifierR1
	modifierL2
	modifierR2
)

func NewKeyboardModelChorded(keyboardLayout [][]string) *KeyboardModelChorded {
	modifiers := []string{"", "l1", "r1", "l2", "r2"}
	triggers := []string{"up", "down", "left", "right", "x", "a", "y", "b"}
	chordMap, chordAltMap := getChordMaps()
	chordActiveMap := make([]map[string]string, len(modifiers))

	layoutNumbers := make([][]rune, len(keyboardLayout))
	for i, row := range keyboardLayout {
		layoutNumbers[i] = make([]rune, len(row))
	}
	// static which keys are active for each modifier
	for mi, mod := range modifiers {
		chordActiveMap[mi] = make(map[string]string)
		for _, trigger := range triggers {
			// for each mod/trigger combos, check all keys
			for i, row := range keyboardLayout {
				for j, key := range row {
					// grab mapped key and alr
					var chordMapKey string
					if mod != "" {
						chordMapKey = strings.Join([]string{mod, trigger}, "+")
					} else {
						chordMapKey = trigger
					}
					chordKey := chordMap[chordMapKey]
					chordAltKey := chordAltMap[chordMapKey]

					if string(chordKey) == strings.ToLower(key) {
						// cache location of key in layout
						chordActiveMap[mi][fmt.Sprintf("%d,%d", i, j)] = trigger
						// make new layout of number keys
						layoutNumbers[i][j] = chordAltKey
					} else {
						if specialKey, ok := specialKeyMap[key]; ok && chordKey == specialKey {
							chordActiveMap[mi][fmt.Sprintf("%d,%d", i, j)] = trigger
						}
					}
				}
			}
		}
	}

	return &KeyboardModelChorded{
		triggers:       triggers,
		layoutNumbers:  layoutNumbers,
		chordMap:       chordMap,
		chordAltMap:    chordAltMap,
		chordActiveMap: chordActiveMap,
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

func getChordMaps() (map[string]rune, map[string]rune) {
	chordMap := make(map[string]rune)
	chordMap["left"] = 'a'
	chordMap["up"] = 's'
	chordMap["down"] = 'd'
	chordMap["right"] = 'f'
	chordMap["x"] = 'h'
	chordMap["a"] = 'j'
	chordMap["y"] = 'k'
	chordMap["b"] = 'l'

	chordMap["l1+left"] = 'q'
	chordMap["l1+up"] = 'w'
	chordMap["l1+down"] = 'e'
	chordMap["l1+right"] = 'r'
	chordMap["l1+x"] = 'u'
	chordMap["l1+a"] = 'i'
	chordMap["l1+y"] = 'o'
	chordMap["l1+b"] = 'p'

	// R1 modifier - next 8 letters
	chordMap["r1+left"] = 'z'
	chordMap["r1+up"] = 'x'
	chordMap["r1+down"] = 'c'
	chordMap["r1+right"] = 'v'
	chordMap["r1+x"] = 'g'
	chordMap["r1+a"] = 'b'
	chordMap["r1+y"] = 'n'
	chordMap["r1+b"] = 'm'

	chordMap["l2+left"] = 't'
	chordMap["l2+up"] = specialKeyShift
	chordMap["l2+down"] = specialKeySpace
	chordMap["l2+right"] = 'y'
	chordMap["l2+x"] = specialKeyDelete
	chordMap["l2+a"] = specialKeyEnter
	chordMap["l2+y"] = specialKeyNumbers
	chordMap["l2+b"] = specialKeyClose

	chordAltMap := make(map[string]rune)
	chordAltMap["left"] = '1'
	chordAltMap["up"] = '2'
	chordAltMap["down"] = '3'
	chordAltMap["right"] = '4'
	chordAltMap["x"] = '5'
	chordAltMap["a"] = '6'
	chordAltMap["y"] = '7'
	chordAltMap["b"] = '8'

	chordAltMap["l1+left"] = ' '
	chordAltMap["l1+up"] = ' '
	chordAltMap["l1+down"] = ' '
	chordAltMap["l1+right"] = ' '
	chordAltMap["l1+x"] = ' '
	chordAltMap["l1+a"] = ' '
	chordAltMap["l1+y"] = ' '
	chordAltMap["l1+b"] = ' '

	// R1 modifier - next 8 letters
	chordAltMap["r1+left"] = ' '
	chordAltMap["r1+up"] = ' '
	chordAltMap["r1+down"] = ' '
	chordAltMap["r1+right"] = ' '
	chordAltMap["r1+x"] = ' '
	chordAltMap["r1+a"] = ' '
	chordAltMap["r1+y"] = ' '
	chordAltMap["r1+b"] = ' '

	chordAltMap["l2+left"] = ' '
	chordAltMap["l2+up"] = ' '
	chordAltMap["l2+down"] = ' '
	chordAltMap["l2+right"] = ' '
	chordAltMap["l2+x"] = ' '
	chordAltMap["l2+a"] = ' '
	chordAltMap["l2+y"] = ' '
	chordAltMap["l2+b"] = ' '
	return chordMap, chordAltMap
}

func (m *KeyboardModelChorded) getModifierString() string {
	switch m.modifier {
	case modifierL1:
		return "l1"
	case modifierR1:
		return "r1"
	case modifierL2:
		return "l2"
	case modifierR2:
		return "r2"
	}
	return ""
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
	// Build chord string
	modStr := m.getModifierString()
	var triggerStr string

	switch triggerKey {
	case keyDPadUp:
		triggerStr = "up"
	case keyDPadDown:
		triggerStr = "down"
	case keyDPadLeft:
		triggerStr = "left"
	case keyDPadRight:
		triggerStr = "right"
	case keyA:
		triggerStr = "a"
	case keyB:
		triggerStr = "b"
	case keyX:
		triggerStr = "x"
	case keyY:
		triggerStr = "y"
	}

	var chordKey string
	if modStr == "" {
		chordKey = triggerStr
	} else {
		chordKey = modStr + "+" + triggerStr
	}

	// Execute the chord
	if char, exists := m.chordMap[chordKey]; exists {
		switch char {
		case specialKeyShift:
			m.modeShift = !m.modeShift
			slog.Info("switching mode shift", "shift", m.modeShift)
		case specialKeyNumbers:
			m.modeNumbers = !m.modeNumbers
			slog.Info("switching mode numbers", "numbers", m.modeNumbers)
		case specialKeyClose:
			slog.Info("sending close")
		case specialKeyEnter:
			slog.Info("sending enter")
		case specialKeyDelete:
			// send delete
			slog.Info("sending delete")
		case specialKeySpace:
			// send space
			slog.Info("sending space")
		default:
			slog.Info("sending key", "key", char)
		}
	}
}

func (m *KeyboardModelChorded) RenderKey(i, j int, key string) (string, string, string) {
	// Check if any modifiers are active
	if len(key) == 1 {
		if m.modeNumbers {
			key = string(m.layoutNumbers[i][j])
		} else if !m.modeShift {
			key = strings.ToLower(key)
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
