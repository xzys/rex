package server

import (
	"log/slog"
)

const (
	specialKeySpace   = ' '
	specialKeyShift   = 'S'
	specialKeyDelete  = 'D'
	specialKeyNumbers = '1'
	specialKeyClose   = 'X'
	specialKeyEnter   = 'E'
)

// ChordExecutor handles chord execution and state management
type ChordExecutor struct {
	modeShift   bool
	modeNumbers bool
	chordMap    map[int]rune
}

// NewChordExecutor creates a new chord executor
func NewChordExecutor() *ChordExecutor {
	chordMap := getChordMaps()
	return &ChordExecutor{
		modeShift:   false,
		modeNumbers: false,
		chordMap:    chordMap,
	}
}

// encodeChord encodes modifier (lower 4 bits) and trigger (upper 4 bits) into a single int
func encodeChord(modifier, trigger int) int {
	return modifier | (trigger << 4)
}

// ExecuteChord executes a chord based on modifier and trigger
func (e *ChordExecutor) ExecuteChord(modifier, trigger int) error {
	chordKey := encodeChord(modifier, trigger)

	if char, exists := e.chordMap[chordKey]; exists {
		switch char {
		case specialKeyShift:
			e.modeShift = !e.modeShift
			slog.Info("switching mode shift", "shift", e.modeShift)
		case specialKeyNumbers:
			e.modeNumbers = !e.modeNumbers
			slog.Info("switching mode numbers", "numbers", e.modeNumbers)
		case specialKeyClose:
			slog.Info("sending close")
		case specialKeyEnter:
			slog.Info("sending enter")
		case specialKeyDelete:
			slog.Info("sending delete")
		case specialKeySpace:
			slog.Info("sending space")
		default:
			slog.Info("sending key", "key", char)
		}
	}
	return nil
}

// GetState returns the current state
func (e *ChordExecutor) GetState() (bool, bool) {
	return e.modeShift, e.modeNumbers
}

// getChordMaps returns the chord mapping
func getChordMaps() map[int]rune {
	const (
		modifierNone = iota
		modifierL1
		modifierR1
		modifierL2
		modifierR2
	)

	const (
		keyDPadUp = iota
		keyDPadDown
		keyDPadLeft
		keyDPadRight
		keyX
		keyA
		keyY
		keyB
	)

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

	return chordMap
}

