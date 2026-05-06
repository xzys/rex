package keyboard

import (
	"testing"

	"github.com/xrfr/t-rex/v2/protocol"
)

func TestFrontChordedModel(t *testing.T) {
	// Mock layout
	layout := [][]string{
		{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"},
		{"a", "s", "d", "f", "g", "h", "j", "k", "l"},
		{"z", "x", "c", "v", "b", "n", "m"},
	}
	model := NewKeyboardModelFrontChorded(layout)

	tests := []struct {
		name string
		ops  []struct {
			key       int
			isKeyDown bool
		}
		expected *string
	}{
		{
			name: "Left Mod Left + X -> q",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyDPadLeft, true}, // Mod
				{protocol.KeyX, true},        // Trigger
			},
			expected: ptrString("q"),
		},
		{
			name: "Left Mod Left + Y -> w",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyDPadLeft, true},
				{protocol.KeyY, true},
			},
			expected: ptrString("w"),
		},
		{
			name: "Left Mod Up + X -> a",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyDPadUp, true},
				{protocol.KeyX, true},
			},
			expected: ptrString("a"),
		},
		{
			name: "Right Mod X + Left -> u",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyX, true},        // Mod
				{protocol.KeyDPadLeft, true}, // Trigger
			},
			expected: ptrString("u"),
		},
		{
			name: "Right Mod A + Right -> Space",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyA, true},         // Mod
				{protocol.KeyDPadRight, true}, // Trigger
			},
			expected: ptrString(" "),
		},
		{
			name: "Numbers Mode Toggle",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyDPadRight, true}, // Mod
				{protocol.KeyX, true},         // Trigger (Numbers) -> Toggle Numbers
				{protocol.KeyDPadRight, false},
				{protocol.KeyX, false},

				// Now try a number chord
				{protocol.KeyDPadLeft, true}, // Mod Left
				{protocol.KeyX, true},        // Trigger X -> Q in letters, 1 in numbers
			},
			expected: ptrString("1"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset model state between tests if needed, or create new model
			model = NewKeyboardModelFrontChorded(layout)

			var result *string
			for _, op := range tc.ops {
				res := model.HandleKey(op.key, op.isKeyDown)
				if res != nil {
					result = res
				}
			}

			if tc.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", *result)
				}
			} else {
				if result == nil {
					t.Errorf("expected %v, got nil", *tc.expected)
				} else if *result != *tc.expected {
					t.Errorf("expected %v, got %v", *tc.expected, *result)
				}
			}
		})
	}
}

func TestFrontChordedModel_RenderKey(t *testing.T) {
	layout := [][]string{
		{"q", "w", "e", "r"},
	}
	model := NewKeyboardModelFrontChorded(layout)

	// Hold modifier KeyDPadLeft (left matrix first row)
	model.HandleKey(protocol.KeyDPadLeft, true)

	// In this state:
	// KeyDPadLeft + KeyX -> 'q'
	// KeyDPadLeft + KeyY -> 'w'
	// KeyDPadLeft + KeyB -> 'e'
	// KeyDPadLeft + KeyA -> 'r'

	// RenderKey(0, 0) -> 'q'. Should be highlighted with trigger 'x' color.
	// Trigger 'x' -> "4", "4" (Blue/Green)
	fg, bg, text := model.RenderKey(0, 0, "q")
	if fg != "4" || bg != "4" {
		t.Errorf("expected highlight for 'q' with mod Left (trigger X), got fg=%s bg=%s", fg, bg)
	}
	if text != "q" {
		t.Errorf("expected text 'q', got %s", text)
	}

	// RenderKey(0, 1) -> 'w'. Trigger 'y'.
	// Trigger 'y' -> "2", "2" (Yellow/Orange)
	fg, bg, text = model.RenderKey(0, 1, "w")
	if fg != "2" || bg != "2" {
		t.Errorf("expected highlight for 'w' with mod Left (trigger Y), got fg=%s bg=%s", fg, bg)
	}
}

func ptrString(s string) *string {
	return &s
}
