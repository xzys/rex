package keyboard

import (
	"log/slog"
	"strings"

	"github.com/xrfr/t-rex/v2/protocol"
)

type KeyboardModelFrontChorded struct {
	// Current modifier state
	modifier       int
	modeNumbers    bool
	layoutNumbers  [][]rune
	chordMap       map[int]rune
	chordAltMap    map[int]rune
	chordActiveMap []map[int]string
}

const frontModifierNone = -1

func packKey(row, col int) int {
	return (row << 8) | col
}

func NewKeyboardModelFrontChorded(keyboardLayout [][]string) *KeyboardModelFrontChorded {
	// Modifiers are the 8 front keys
	modifiers := []int{
		protocol.KeyDPadUp, protocol.KeyDPadDown, protocol.KeyDPadLeft, protocol.KeyDPadRight,
		protocol.KeyX, protocol.KeyA, protocol.KeyY, protocol.KeyB,
	}

	// Triggers can be any of the 8 front keys
	triggers := []int{
		protocol.KeyDPadUp, protocol.KeyDPadDown, protocol.KeyDPadLeft, protocol.KeyDPadRight,
		protocol.KeyX, protocol.KeyA, protocol.KeyY, protocol.KeyB,
	}

	triggerNames := map[int]string{
		protocol.KeyDPadUp:    "up",
		protocol.KeyDPadDown:  "down",
		protocol.KeyDPadLeft:  "left",
		protocol.KeyDPadRight: "right",
		protocol.KeyX:         "x",
		protocol.KeyA:         "a",
		protocol.KeyY:         "y",
		protocol.KeyB:         "b",
	}

	chordMap, chordAltMap := getFrontChordMaps()

	// Initialize chordActiveMap for visual feedback
	// Map: modifier -> packedKey(row,col) -> triggerName
	chordActiveMap := make([]map[int]string, 256) // Using 256 to cover key codes safely

	// Pre-fill numbers layout with spaces
	layoutNumbers := make([][]rune, len(keyboardLayout))
	for i, row := range keyboardLayout {
		layoutNumbers[i] = make([]rune, len(row))
		for j := range row {
			layoutNumbers[i][j] = ' '
		}
	}

	// Populate chordActiveMap and layoutNumbers
	for _, mod := range modifiers {
		chordActiveMap[mod] = make(map[int]string)

		for _, trigger := range triggers {
			// Skip if modifier and trigger are on the same hand (simplified check)
			// Actually, let's just iterate all valid chords

			// encode the chord key
			chordMapKey := encodeChord(mod, trigger)

			// Check if this chord exists
			chordKey, exists := chordMap[chordMapKey]
			chordAltKey, chordAltExists := chordAltMap[chordMapKey]

			if !exists && !chordAltExists {
				continue
			}

			// Find which key on the visual keyboard corresponds to this chord
			for i, row := range keyboardLayout {
				for j, key := range row {
					if exists && string(chordKey) == strings.ToLower(key) {
						chordActiveMap[mod][packKey(i, j)] = triggerNames[trigger]
						if chordAltExists {
							layoutNumbers[i][j] = chordAltKey
						}
					} else if specialKey, ok := specialKeyMap[key]; ok && exists && chordKey == specialKey {
						chordActiveMap[mod][packKey(i, j)] = triggerNames[trigger]
						if chordAltExists {
							layoutNumbers[i][j] = chordAltKey
						}
					}
				}
			}
		}
	}

	return &KeyboardModelFrontChorded{
		modifier:       frontModifierNone,
		layoutNumbers:  layoutNumbers,
		chordMap:       chordMap,
		chordAltMap:    chordAltMap,
		chordActiveMap: chordActiveMap,
	}
}

func getFrontChordMaps() (map[int]rune, map[int]rune) {
	chordMap := make(map[int]rune)
	chordAltMap := make(map[int]rune)

	// The 4x8 front-chorded grid is two 4x4 matrices.
	// Left matrix: D-pad modifiers (left, up, down, right) + XYBA triggers.
	// Right matrix: XYBA modifiers + D-pad triggers (left, up, down, right).

	// Left hand modifiers, XYBA trigger columns.
	modLeft := protocol.KeyDPadLeft
	chordMap[encodeChord(modLeft, protocol.KeyX)] = 'q'
	chordMap[encodeChord(modLeft, protocol.KeyY)] = 'w'
	chordMap[encodeChord(modLeft, protocol.KeyB)] = 'e'
	chordMap[encodeChord(modLeft, protocol.KeyA)] = 'r'

	modUp := protocol.KeyDPadUp
	chordMap[encodeChord(modUp, protocol.KeyX)] = 'a'
	chordMap[encodeChord(modUp, protocol.KeyY)] = 's'
	chordMap[encodeChord(modUp, protocol.KeyB)] = 'd'
	chordMap[encodeChord(modUp, protocol.KeyA)] = 'f'

	modDown := protocol.KeyDPadDown
	chordMap[encodeChord(modDown, protocol.KeyX)] = 'z'
	chordMap[encodeChord(modDown, protocol.KeyY)] = 'x'
	chordMap[encodeChord(modDown, protocol.KeyB)] = 'c'
	chordMap[encodeChord(modDown, protocol.KeyA)] = 'v'

	modRight := protocol.KeyDPadRight
	chordMap[encodeChord(modRight, protocol.KeyX)] = protocol.SpecialKeyNumbers
	chordMap[encodeChord(modRight, protocol.KeyY)] = protocol.SpecialKeyShift
	chordMap[encodeChord(modRight, protocol.KeyB)] = 'g'
	chordMap[encodeChord(modRight, protocol.KeyA)] = 't'

	// Right hand modifiers, D-pad trigger columns.
	modX := protocol.KeyX
	chordMap[encodeChord(modX, protocol.KeyDPadLeft)] = 'u'
	chordMap[encodeChord(modX, protocol.KeyDPadUp)] = 'i'
	chordMap[encodeChord(modX, protocol.KeyDPadDown)] = 'o'
	chordMap[encodeChord(modX, protocol.KeyDPadRight)] = 'p'

	modY := protocol.KeyY
	chordMap[encodeChord(modY, protocol.KeyDPadLeft)] = 'h'
	chordMap[encodeChord(modY, protocol.KeyDPadUp)] = 'j'
	chordMap[encodeChord(modY, protocol.KeyDPadDown)] = 'k'
	chordMap[encodeChord(modY, protocol.KeyDPadRight)] = 'l'

	modB := protocol.KeyB
	chordMap[encodeChord(modB, protocol.KeyDPadLeft)] = 'b'
	chordMap[encodeChord(modB, protocol.KeyDPadUp)] = 'n'
	chordMap[encodeChord(modB, protocol.KeyDPadDown)] = 'm'
	chordMap[encodeChord(modB, protocol.KeyDPadRight)] = protocol.SpecialKeyMic

	modA := protocol.KeyA
	chordMap[encodeChord(modA, protocol.KeyDPadLeft)] = 'y'
	chordMap[encodeChord(modA, protocol.KeyDPadUp)] = protocol.SpecialKeyEnter
	chordMap[encodeChord(modA, protocol.KeyDPadDown)] = protocol.SpecialKeyDelete
	chordMap[encodeChord(modA, protocol.KeyDPadRight)] = protocol.SpecialKeySpace

	// Numbers mode follows the QWERTY number row: qwertyuiop -> 1234567890.
	chordAltMap[encodeChord(modLeft, protocol.KeyX)] = '1'      // q
	chordAltMap[encodeChord(modLeft, protocol.KeyY)] = '2'      // w
	chordAltMap[encodeChord(modLeft, protocol.KeyB)] = '3'      // e
	chordAltMap[encodeChord(modLeft, protocol.KeyA)] = '4'      // r
	chordAltMap[encodeChord(modRight, protocol.KeyA)] = '5'     // t
	chordAltMap[encodeChord(modA, protocol.KeyDPadLeft)] = '6'  // y
	chordAltMap[encodeChord(modX, protocol.KeyDPadLeft)] = '7'  // u
	chordAltMap[encodeChord(modX, protocol.KeyDPadUp)] = '8'    // i
	chordAltMap[encodeChord(modX, protocol.KeyDPadDown)] = '9'  // o
	chordAltMap[encodeChord(modX, protocol.KeyDPadRight)] = '0' // p

	return chordMap, chordAltMap
}

func (m *KeyboardModelFrontChorded) HandleKey(key int, isKeyDown bool) *string {
	// State Machine:
	// Idle: No keys held.
	// Modifier Held: User holds one of the 8 front keys.
	// Trigger Press: User taps a key on the opposite side.
	// Release: Modifier is cleared on key up.

	if !isKeyDown {
		// Key Up
		if key == m.modifier {
			slog.Debug("clearing modifier", "modifier", m.modifier)
			m.modifier = frontModifierNone
		}
		return nil
	}

	// Key Down
	if m.modifier == frontModifierNone {
		// First key pressed becomes modifier
		m.modifier = key
		slog.Debug("setting modifier", "modifier", m.modifier)
		return nil
	}

	// Modifier is already held, this is a trigger
	trigger := key
	result := m.getChord(trigger)

	// Clear modifier after successful chord so next key starts fresh
	// (keyboard input device only sends key-down, no key-up to clear)
	if result != nil {
		m.modifier = frontModifierNone
	}

	return result
}

func (m *KeyboardModelFrontChorded) getChord(triggerKey int) *string {
	chordKey := encodeChord(m.modifier, triggerKey)

	var char rune
	var exists bool
	if m.modeNumbers {
		char, exists = m.chordAltMap[chordKey]
		// Fallback to normal map if not in alt map
		if !exists {
			char, exists = m.chordMap[chordKey]
		}
	} else {
		char, exists = m.chordMap[chordKey]
	}

	if !exists {
		slog.Debug("chord not found", "modifier", m.modifier, "trigger", triggerKey)
		return nil
	}

	// Update local state based on chord
	switch char {
	case protocol.SpecialKeyShift: // Used as Shift/Numbers toggle in plan?
		// Plan says "Shift/Numbers". Let's assume it toggles numbers mode?
		// Or maybe Shift for caps?
		// Let's make it toggle Numbers for now as per "Shift/Numbers" label.
		// Or maybe it's Shift AND Numbers?
		// Let's implement Numbers toggle.
		slog.Debug("toggling numbers mode", "mode", m.modeNumbers)
		m.modeNumbers = !m.modeNumbers
		return nil
	case protocol.SpecialKeyNumbers:
		slog.Debug("toggling numbers mode", "mode", m.modeNumbers)
		m.modeNumbers = !m.modeNumbers
		return nil
	}

	s := string(char)
	// Simple Shift logic if we had a separate Shift key, but here we might just output uppercase if needed?
	// For now, let's just output the char.

	slog.Info("send chord", "chord", s)
	return &s
}

func (m *KeyboardModelFrontChorded) RenderKey(i, j int, key string) (string, string, string) {
	// Check if any modifiers are active
	if len(key) == 1 {
		if m.modeNumbers {
			key = string(m.layoutNumbers[i][j])
		}
		// else if m.modeShift {
		// 	key = strings.ToUpper(key)
		// }
	}

	if m.modifier != frontModifierNone {
		if triggerName, exists := m.chordActiveMap[m.modifier][packKey(i, j)]; exists {
			// Highlight keys based on trigger
			switch triggerName {
			case "x":
				return "4", "4", key // Blue/Green
			case "y":
				return "2", "2", key // Yellow/Orange
			case "a":
				return "1", "1", key // Red
			case "b":
				return "3", "3", key // Purple/Blue
			case "up", "down", "left", "right":
				return "7", "7", key // White
			}
		}
	}

	return "7", "0", key
}
