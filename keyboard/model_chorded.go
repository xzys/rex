package keyboard

import (
	"fmt"
	"log"
	"strings"
)

type KeyboardModelChorded struct {
	// Current modifier state
	modifier       int
	triggers       []string
	chordMap       map[string]rune
	chordActiveMap []map[string]bool
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
	chordMap := getChordMap()
	chordActiveMap := make([]map[string]bool, len(modifiers))

	// static which keys are active for each modifier
	for mi, mod := range modifiers {
		chordActiveMap[mi] = make(map[string]bool)
		for _, trigger := range triggers {
			// for each mod/trigger combos, check all keys
			for i, row := range keyboardLayout {
				for j, key := range row {
					var chordKey rune
					if mod != "" {
						chordKey = chordMap[strings.Join([]string{mod, trigger}, "+")]
					} else {
						chordKey = chordMap[trigger]
					}
					if string(chordKey) == strings.ToLower(key) {
						chordActiveMap[mi][fmt.Sprintf("%d,%d", i, j)] = true
					}
				}
			}
		}
	}

	return &KeyboardModelChorded{
		triggers:       triggers,
		chordMap:       chordMap,
		chordActiveMap: chordActiveMap,
	}
}

func getChordMap() map[string]rune {
	chordMap := make(map[string]rune)
	chordMap["up"] = 'a'
	chordMap["down"] = 's'
	chordMap["left"] = 'd'
	chordMap["right"] = 'f'
	chordMap["x"] = 'h'
	chordMap["a"] = 'j'
	chordMap["y"] = 'k'
	chordMap["b"] = 'l'

	chordMap["l1+up"] = 'q'
	chordMap["l1+down"] = 'w'
	chordMap["l1+left"] = 'e'
	chordMap["l1+right"] = 'r'
	chordMap["l1+x"] = 'u'
	chordMap["l1+a"] = 'i'
	chordMap["l1+y"] = 'o'
	chordMap["l1+b"] = 'p'

	// R1 modifier - next 8 letters
	chordMap["r1+up"] = 'z'
	chordMap["r1+down"] = 'x'
	chordMap["r1+left"] = 'c'
	chordMap["r1+right"] = 'v'
	chordMap["r1+x"] = 'g'
	chordMap["r1+a"] = 'b'
	chordMap["r1+y"] = 'n'
	chordMap["r1+b"] = 'm'

	chordMap["l2+up"] = ' '   // shift
	chordMap["l2+down"] = ' ' // space
	chordMap["l2+left"] = 't'
	chordMap["l2+right"] = 'y'
	chordMap["l2+x"] = ' ' // delete
	chordMap["l2+a"] = ' ' // enter
	chordMap["l2+y"] = ' ' // numbers?
	chordMap["l2+b"] = ' ' // close
	return chordMap
}

func (m *KeyboardModelChorded) getModifierString() string {
	switch m.modifier {
	case modifierL1:
		return "l1"
	case modifierR1:
		return "r1"
	case modifierL2:
		return "l2"
	}
	return ""
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
			m.modifier = modifierNone
		}
	// Trigger keys - complete the chord
	case keyDPadUp, keyDPadDown, keyDPadLeft, keyDPadRight, keyA, keyB, keyX, keyY:
		m.executeChord(key)
		// Reset modifiers after trigger
		m.modifier = modifierNone
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
		log.Println(string(char))
	}
}

func (m *KeyboardModelChorded) RenderKey(i, j int) (string, string) {
	// Check if any modifiers are active
	if m.chordActiveMap[m.modifier][fmt.Sprintf("%d,%d", i, j)] {
		return "4", "4"
	}

	return "0", "0"
}
