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
	model := NewKeyboardModelFrontChorded(layout, nil)

	tests := []struct {
		name     string
		ops      []struct {
			key       int
			isKeyDown bool
		}
		expected *string
	}{
		{
			name: "Left Mod Up + Y -> q",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyDPadUp, true}, // Mod
				{protocol.KeyY, true},      // Trigger
			},
			expected: ptrString("q"),
		},
		{
			name: "Left Mod Up + X -> w",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyDPadUp, true},
				{protocol.KeyX, true},
			},
			expected: ptrString("w"),
		},
		{
			name: "Left Mod Left + Y -> a",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyDPadLeft, true},
				{protocol.KeyY, true},
			},
			expected: ptrString("a"),
		},
		{
			name: "Right Mod Y + Up -> u",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyY, true},      // Mod
				{protocol.KeyDPadUp, true}, // Trigger
			},
			expected: ptrString("u"),
		},
		{
			name: "Right Mod X + Left -> Space",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyX, true},        // Mod
				{protocol.KeyDPadLeft, true}, // Trigger
			},
			expected: ptrString(" "),
		},
		{
			name: "Numbers Mode Toggle",
			ops: []struct {
				key       int
				isKeyDown bool
			}{
				{protocol.KeyX, true},       // Mod
				{protocol.KeyDPadUp, true},  // Trigger (Shift/Numbers) -> Toggle Numbers
				{protocol.KeyX, false},      // Release Mod
				{protocol.KeyDPadUp, false}, // Release Trigger
				
				// Now try a number chord
				{protocol.KeyDPadUp, true}, // Mod Up
				{protocol.KeyY, true},      // Trigger Y -> Q in letters, 1 in numbers
			},
			expected: ptrString("1"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset model state between tests if needed, or create new model
			model = NewKeyboardModelFrontChorded(layout, nil)
			
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
	model := NewKeyboardModelFrontChorded(layout, nil)

	// Hold modifier KeyDPadUp (Left Mod Up)
	model.HandleKey(protocol.KeyDPadUp, true)

	// In this state:
	// KeyDPadUp + KeyY -> 'q'
	// KeyDPadUp + KeyX -> 'w'
	// KeyDPadUp + KeyB -> 'e'
	// KeyDPadUp + KeyA -> 'r'

	// RenderKey(0, 0) -> 'q'. Should be highlighted with trigger 'y' color.
	// Trigger 'y' -> "2", "2" (Yellow/Orange)
	fg, bg, text := model.RenderKey(0, 0, "q")
	if fg != "2" || bg != "2" {
		t.Errorf("expected highlight for 'q' with mod Up (trigger Y), got fg=%s bg=%s", fg, bg)
	}
	if text != "q" {
		t.Errorf("expected text 'q', got %s", text)
	}

	// RenderKey(0, 1) -> 'w'. Trigger 'x'.
	// Trigger 'x' -> "4", "4" (Blue/Green)
	fg, bg, text = model.RenderKey(0, 1, "w")
	if fg != "4" || bg != "4" {
		t.Errorf("expected highlight for 'w' with mod Up (trigger X), got fg=%s bg=%s", fg, bg)
	}
}

func TestFrontChordedModel_Autocomplete(t *testing.T) {
	layout := [][]string{
		{"q", "w", "e", "r", "t", "y", "u", "i", "o", "p"},
		{"a", "s", "d", "f", "g", "h", "j", "k", "l"},
		{"z", "x", "c", "v", "b", "n", "m"},
	}

	// Build a small test dictionary
	dict := NewTrie()
	words := []string{"the", "they", "there", "then", "this", "think", "to", "that"}
	for i, w := range words {
		dict.Insert(w, i)
	}

	// Helper: type a chord and release both keys
	typeChord := func(model *KeyboardModelFrontChorded, mod, trigger int) *string {
		model.HandleKey(mod, true)
		result := model.HandleKey(trigger, true)
		model.HandleKey(trigger, false)
		model.HandleKey(mod, false)
		return result
	}

	t.Run("suggestions appear after typing", func(t *testing.T) {
		model := NewKeyboardModelFrontChorded(layout, dict)

		// Type 't' (DPad Right + Y = 't')
		typeChord(model, protocol.KeyDPadRight, protocol.KeyY)

		suggestions := model.Suggestions()
		if suggestions[0] != "the" {
			t.Errorf("expected first suggestion 'the', got %q", suggestions[0])
		}
		if model.CurrentWord() != "t" {
			t.Errorf("expected currentWord 't', got %q", model.CurrentWord())
		}
	})

	t.Run("suggestions narrow with more typing", func(t *testing.T) {
		model := NewKeyboardModelFrontChorded(layout, dict)

		// Type 'th' -> t then h
		typeChord(model, protocol.KeyDPadRight, protocol.KeyY)  // t
		typeChord(model, protocol.KeyA, protocol.KeyDPadUp)     // h

		suggestions := model.Suggestions()
		if suggestions[0] != "the" {
			t.Errorf("expected first suggestion 'the', got %q", suggestions[0])
		}
		if model.CurrentWord() != "th" {
			t.Errorf("expected currentWord 'th', got %q", model.CurrentWord())
		}
	})

	t.Run("L1 selects first suggestion", func(t *testing.T) {
		model := NewKeyboardModelFrontChorded(layout, dict)

		// Type 'th'
		typeChord(model, protocol.KeyDPadRight, protocol.KeyY) // t
		typeChord(model, protocol.KeyA, protocol.KeyDPadUp)    // h

		// Press L1 to select first suggestion ("the")
		result := model.HandleKey(protocol.KeyL1, true)
		if result == nil {
			t.Fatal("expected non-nil result from L1")
		}
		// Should return remaining chars + space: "e "
		if *result != "e " {
			t.Errorf("expected 'e ', got %q", *result)
		}
		// currentWord and suggestions should be cleared
		if model.CurrentWord() != "" {
			t.Errorf("expected empty currentWord, got %q", model.CurrentWord())
		}
		if model.Suggestions() != [4]string{} {
			t.Errorf("expected empty suggestions, got %v", model.Suggestions())
		}
	})

	t.Run("R1 selects third suggestion", func(t *testing.T) {
		model := NewKeyboardModelFrontChorded(layout, dict)

		// Type 'th'
		typeChord(model, protocol.KeyDPadRight, protocol.KeyY) // t
		typeChord(model, protocol.KeyA, protocol.KeyDPadUp)    // h

		suggestions := model.Suggestions()
		if suggestions[2] == "" {
			t.Fatal("expected non-empty third suggestion")
		}
		expectedWord := suggestions[2]

		result := model.HandleKey(protocol.KeyR1, true)
		if result == nil {
			t.Fatal("expected non-nil result from R1")
		}
		expectedRemaining := expectedWord[2:] + " " // skip "th"
		if *result != expectedRemaining {
			t.Errorf("expected %q, got %q", expectedRemaining, *result)
		}
	})

	t.Run("space clears currentWord", func(t *testing.T) {
		model := NewKeyboardModelFrontChorded(layout, dict)

		// Type 't'
		typeChord(model, protocol.KeyDPadRight, protocol.KeyY)
		if model.CurrentWord() != "t" {
			t.Fatalf("expected 't', got %q", model.CurrentWord())
		}

		// Type space (X + DPadLeft)
		typeChord(model, protocol.KeyX, protocol.KeyDPadLeft)
		if model.CurrentWord() != "" {
			t.Errorf("expected empty currentWord after space, got %q", model.CurrentWord())
		}
	})

	t.Run("no suggestions without dictionary", func(t *testing.T) {
		model := NewKeyboardModelFrontChorded(layout, nil)

		typeChord(model, protocol.KeyDPadRight, protocol.KeyY) // t
		if model.Suggestions() != [4]string{} {
			t.Errorf("expected no suggestions without dictionary")
		}
	})

	t.Run("shoulder button with no suggestion returns nil", func(t *testing.T) {
		model := NewKeyboardModelFrontChorded(layout, dict)

		// Press L1 without typing anything
		result := model.HandleKey(protocol.KeyL1, true)
		if result != nil {
			t.Errorf("expected nil, got %q", *result)
		}
	})
}

func ptrString(s string) *string {
	return &s
}
