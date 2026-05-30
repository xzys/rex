package keyboard

import (
	"log/slog"

	"github.com/xrfr/t-rex/v2/protocol"
)

// encodeChord packs (modifier, trigger) into a single int key for the chord
// lookup map. modifier occupies the low 4 bits, trigger the upper bits.
func encodeChord(modifier, trigger int) int {
	return modifier | (trigger << 4)
}

type KeyboardModelFrontChorded struct {
	// Current modifier state
	modifier int
	layers   layerState
	// chordMap maps a packed (modifier, trigger) chord to the base on-screen
	// key label it selects (a letter like "q" or a special label like "SPC"/
	// "123"/"SFT"). Layer transforms (numbers/shift) are applied afterward by
	// layerState, so the chord set itself is layer-agnostic.
	chordMap map[int]string
	// chordActiveMap drives visual feedback: modifier -> packedKey(row,col) ->
	// trigger name, so RenderKey can highlight the keys reachable from the held
	// modifier and color them by which trigger reaches them.
	chordActiveMap []map[int]string
}

const frontModifierNone = -1

func packKey(row, col int) int {
	return (row << 8) | col
}

func NewKeyboardModelFrontChorded(keyboardLayout [][]string) *KeyboardModelFrontChorded {
	// Modifiers and triggers are the 8 front keys.
	frontKeys := []int{
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

	chordMap := frontChordLabels()

	// Map each chord onto its visual key so RenderKey can highlight reachable
	// keys while a modifier is held. Indexed by raw key code (256 covers all
	// front-key codes safely).
	chordActiveMap := make([]map[int]string, 256)
	for _, mod := range frontKeys {
		chordActiveMap[mod] = make(map[int]string)
		for _, trigger := range frontKeys {
			label, ok := chordMap[encodeChord(mod, trigger)]
			if !ok {
				continue
			}
			for i, row := range keyboardLayout {
				for j, key := range row {
					if key == label {
						chordActiveMap[mod][packKey(i, j)] = triggerNames[trigger]
					}
				}
			}
		}
	}

	return &KeyboardModelFrontChorded{
		modifier:       frontModifierNone,
		chordMap:       chordMap,
		chordActiveMap: chordActiveMap,
	}
}

// frontChordLabels returns the chord -> base-label map. The 4x8 front-chorded
// grid is two 4x4 matrices: the left matrix uses D-pad modifiers + XYBA
// triggers, the right matrix uses XYBA modifiers + D-pad triggers.
func frontChordLabels() map[int]string {
	m := make(map[int]string)

	// Left hand modifiers, XYBA trigger columns.
	modLeft := protocol.KeyDPadLeft
	m[encodeChord(modLeft, protocol.KeyX)] = "q"
	m[encodeChord(modLeft, protocol.KeyY)] = "w"
	m[encodeChord(modLeft, protocol.KeyB)] = "e"
	m[encodeChord(modLeft, protocol.KeyA)] = "r"

	modUp := protocol.KeyDPadUp
	m[encodeChord(modUp, protocol.KeyX)] = "a"
	m[encodeChord(modUp, protocol.KeyY)] = "s"
	m[encodeChord(modUp, protocol.KeyB)] = "d"
	m[encodeChord(modUp, protocol.KeyA)] = "f"

	modDown := protocol.KeyDPadDown
	m[encodeChord(modDown, protocol.KeyX)] = "z"
	m[encodeChord(modDown, protocol.KeyY)] = "x"
	m[encodeChord(modDown, protocol.KeyB)] = "c"
	m[encodeChord(modDown, protocol.KeyA)] = "v"

	modRight := protocol.KeyDPadRight
	m[encodeChord(modRight, protocol.KeyX)] = "123"
	m[encodeChord(modRight, protocol.KeyY)] = "SFT"
	m[encodeChord(modRight, protocol.KeyB)] = "g"
	m[encodeChord(modRight, protocol.KeyA)] = "t"

	// Right hand modifiers, D-pad trigger columns.
	modX := protocol.KeyX
	m[encodeChord(modX, protocol.KeyDPadLeft)] = "u"
	m[encodeChord(modX, protocol.KeyDPadUp)] = "i"
	m[encodeChord(modX, protocol.KeyDPadDown)] = "o"
	m[encodeChord(modX, protocol.KeyDPadRight)] = "p"

	modY := protocol.KeyY
	m[encodeChord(modY, protocol.KeyDPadLeft)] = "h"
	m[encodeChord(modY, protocol.KeyDPadUp)] = "j"
	m[encodeChord(modY, protocol.KeyDPadDown)] = "k"
	m[encodeChord(modY, protocol.KeyDPadRight)] = "l"

	modB := protocol.KeyB
	m[encodeChord(modB, protocol.KeyDPadLeft)] = "b"
	m[encodeChord(modB, protocol.KeyDPadUp)] = "n"
	m[encodeChord(modB, protocol.KeyDPadDown)] = "m"
	m[encodeChord(modB, protocol.KeyDPadRight)] = "MIC"

	modA := protocol.KeyA
	m[encodeChord(modA, protocol.KeyDPadLeft)] = "y"
	m[encodeChord(modA, protocol.KeyDPadUp)] = "ENT"
	m[encodeChord(modA, protocol.KeyDPadDown)] = "DEL"
	m[encodeChord(modA, protocol.KeyDPadRight)] = "SPC"

	return m
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

	// Modifier is already held, this is a trigger.
	out, handled := m.getChord(key)
	if handled {
		// Clear modifier after a recognized chord so the next key starts fresh
		// (the input device only sends key-down, no key-up to clear).
		m.modifier = frontModifierNone
	}
	return out
}

// getChord resolves the chord for the held modifier + `trigger` to the bytes
// sent to the server. The bool reports whether the chord was recognized (so
// the caller can clear the modifier) — distinct from a recognized chord that
// produces no output, like the "123"/"SFT" layer toggles.
func (m *KeyboardModelFrontChorded) getChord(trigger int) (*string, bool) {
	label, ok := m.chordMap[encodeChord(m.modifier, trigger)]
	if !ok {
		slog.Debug("chord not found", "modifier", m.modifier, "trigger", trigger)
		return nil, false
	}
	out := m.layers.resolve(label)
	if out != nil {
		slog.Info("send chord", "chord", *out)
	}
	return out, true
}

func (m *KeyboardModelFrontChorded) RenderKey(i, j int, key string) (string, string, string) {
	glyph := m.layers.glyph(key)

	if m.modifier != frontModifierNone {
		if triggerName, exists := m.chordActiveMap[m.modifier][packKey(i, j)]; exists {
			// Highlight keys based on trigger
			switch triggerName {
			case "x":
				return "4", "4", glyph // Blue/Green
			case "y":
				return "2", "2", glyph // Yellow/Orange
			case "a":
				return "1", "1", glyph // Red
			case "b":
				return "3", "3", glyph // Purple/Blue
			case "up", "down", "left", "right":
				return "7", "7", glyph // White
			}
		}
	}

	if m.layers.active(key) {
		return "3", "3", glyph
	}
	return "7", "0", glyph
}
