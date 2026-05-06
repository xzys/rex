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

	// Helper to map a zone
	// Modifiers: D-Pad (Left Hand)
	// Triggers: ABXY (Right Hand)

	// Left Hand Modifiers

	// Up: Q, W, E, R
	modUp := protocol.KeyDPadUp
	chordMap[encodeChord(modUp, protocol.KeyY)] = 'q'
	chordMap[encodeChord(modUp, protocol.KeyX)] = 'w'
	chordMap[encodeChord(modUp, protocol.KeyB)] = 'e'
	chordMap[encodeChord(modUp, protocol.KeyA)] = 'r'

	// Left: A, S, D, F
	modLeft := protocol.KeyDPadLeft
	chordMap[encodeChord(modLeft, protocol.KeyY)] = 'a'
	chordMap[encodeChord(modLeft, protocol.KeyX)] = 's'
	chordMap[encodeChord(modLeft, protocol.KeyB)] = 'd'
	chordMap[encodeChord(modLeft, protocol.KeyA)] = 'f'

	// Down: Z, X, C, V
	modDown := protocol.KeyDPadDown
	chordMap[encodeChord(modDown, protocol.KeyY)] = 'z'
	chordMap[encodeChord(modDown, protocol.KeyX)] = 'x'
	chordMap[encodeChord(modDown, protocol.KeyB)] = 'c'
	chordMap[encodeChord(modDown, protocol.KeyA)] = 'v'

	// Right: T, G, B, Y
	modRight := protocol.KeyDPadRight
	chordMap[encodeChord(modRight, protocol.KeyY)] = 't'
	chordMap[encodeChord(modRight, protocol.KeyX)] = 'g'
	chordMap[encodeChord(modRight, protocol.KeyB)] = 'b'
	chordMap[encodeChord(modRight, protocol.KeyA)] = 'y'

	// Right Hand Modifiers
	// Triggers: D-Pad (Left Hand)

	// Y (Up): U, I, O, P
	modY := protocol.KeyY
	chordMap[encodeChord(modY, protocol.KeyDPadUp)] = 'u'
	chordMap[encodeChord(modY, protocol.KeyDPadLeft)] = 'i'
	chordMap[encodeChord(modY, protocol.KeyDPadDown)] = 'o'
	chordMap[encodeChord(modY, protocol.KeyDPadRight)] = 'p'

	// X (Left) [Inner Right Zone]: Special Keys
	modX := protocol.KeyX
	chordMap[encodeChord(modX, protocol.KeyDPadUp)] = protocol.SpecialKeyNumbers // Shift/Numbers? Plan says Shift/Numbers
	chordMap[encodeChord(modX, protocol.KeyDPadLeft)] = protocol.SpecialKeySpace
	chordMap[encodeChord(modX, protocol.KeyDPadDown)] = protocol.SpecialKeyEnter
	chordMap[encodeChord(modX, protocol.KeyDPadRight)] = protocol.SpecialKeyDelete // Backspace

	// A (Right): H, J, K, L
	modA := protocol.KeyA
	chordMap[encodeChord(modA, protocol.KeyDPadUp)] = 'h'
	chordMap[encodeChord(modA, protocol.KeyDPadLeft)] = 'j'
	chordMap[encodeChord(modA, protocol.KeyDPadDown)] = 'k'
	chordMap[encodeChord(modA, protocol.KeyDPadRight)] = 'l'

	// B (Down): N, M, Shift, Mic
	modB := protocol.KeyB
	chordMap[encodeChord(modB, protocol.KeyDPadUp)] = 'n'
	chordMap[encodeChord(modB, protocol.KeyDPadLeft)] = 'm'
	chordMap[encodeChord(modB, protocol.KeyDPadDown)] = protocol.SpecialKeyShift
	chordMap[encodeChord(modB, protocol.KeyDPadRight)] = protocol.SpecialKeyMic

	// Numbers Mode Mapping (Populate chordAltMap)
	// I'll map numbers to the QWERTY row equivalent positions if possible, or just 1-0
	// Plan doesn't explicitly define number mapping, but implies "Shift/Numbers" toggles it.
	// Let's assume a standard mapping or just map top row QWERTY to 1-0.

	// QWERTY row is on Mod Up (QWER) and Mod Y (UIOP) + Mod Right (T)
	// Let's map:
	// Mod Up + Y (Q) -> 1
	chordAltMap[encodeChord(modUp, protocol.KeyY)] = '1'
	// Mod Up + X (W) -> 2
	chordAltMap[encodeChord(modUp, protocol.KeyX)] = '2'
	// Mod Up + B (E) -> 3
	chordAltMap[encodeChord(modUp, protocol.KeyB)] = '3'
	// Mod Up + A (R) -> 4
	chordAltMap[encodeChord(modUp, protocol.KeyA)] = '4'

	// Mod Right + Y (T) -> 5
	chordAltMap[encodeChord(modRight, protocol.KeyY)] = '5'

	// Mod Y + Up (U) -> 7 (Wait, standard is 12345 67890)
	// Y (Up) -> U, I, O, P
	// Let's map Y + Up (U) -> 7
	chordAltMap[encodeChord(modY, protocol.KeyDPadUp)] = '7'
	// Y + Left (I) -> 8
	chordAltMap[encodeChord(modY, protocol.KeyDPadLeft)] = '8'
	// Y + Down (O) -> 9
	chordAltMap[encodeChord(modY, protocol.KeyDPadDown)] = '9'
	// Y + Right (P) -> 0
	chordAltMap[encodeChord(modY, protocol.KeyDPadRight)] = '0'

	// Missing 6?
	// Mod Right + X (G) -> 6?
	chordAltMap[encodeChord(modRight, protocol.KeyX)] = '6'

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
