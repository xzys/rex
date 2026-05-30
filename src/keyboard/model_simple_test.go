package keyboard

import (
	"testing"

	"github.com/xrfr/t-rex/v2/protocol"
)

var simpleTestLayout = [][]string{
	{"123", "q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "MIC"},
	{"SFT", "a", "s", "d", "f", "g", "h", "j", "k", "l", "SPC"},
	{"DEL", "z", "x", "c", "v", "b", "n", "m", "ENT"},
}

// pressAt moves the cursor to (row,col) and taps A, returning what the model
// would send to the server.
func pressAt(m *KeyboardModelSimple, row, col int) *string {
	m.selectedRow, m.selectedCol = row, col
	return m.HandleKey(protocol.KeyA, true)
}

func TestSimpleLetter(t *testing.T) {
	m := NewKeyboardModelSimple(simpleTestLayout)
	got := pressAt(m, 0, 1) // q
	if got == nil || *got != "q" {
		t.Fatalf("expected q, got %v", got)
	}
}

func TestSimpleShiftSticky(t *testing.T) {
	m := NewKeyboardModelSimple(simpleTestLayout)
	if got := pressAt(m, 1, 0); got != nil { // SFT toggles, emits nothing
		t.Fatalf("SFT should emit nil, got %v", *got)
	}
	got := pressAt(m, 0, 1) // q -> Q
	if got == nil || *got != "Q" {
		t.Fatalf("expected Q after shift, got %v", got)
	}
	got = pressAt(m, 0, 2) // w -> W (shift is sticky)
	if got == nil || *got != "W" {
		t.Fatalf("expected sticky uppercase W, got %v", got)
	}
	pressAt(m, 1, 0)        // SFT again -> off
	got = pressAt(m, 0, 3) // e -> e
	if got == nil || *got != "e" {
		t.Fatalf("expected lowercase e after toggling shift off, got %v", got)
	}
}

func TestSimpleNumbersToggle(t *testing.T) {
	m := NewKeyboardModelSimple(simpleTestLayout)
	if got := pressAt(m, 0, 0); got != nil { // 123 toggles, emits nothing
		t.Fatalf("123 should emit nil, got %v", *got)
	}
	got := pressAt(m, 0, 1) // q -> 1
	if got == nil || *got != "1" {
		t.Fatalf("expected 1 in numbers mode, got %v", got)
	}
	got = pressAt(m, 1, 1) // a -> @
	if got == nil || *got != "@" {
		t.Fatalf("expected @ in numbers mode, got %v", got)
	}
	// numbers mode is sticky
	got = pressAt(m, 0, 2) // w -> 2
	if got == nil || *got != "2" {
		t.Fatalf("numbers mode should be sticky, got %v", got)
	}
	// toggle back to letters
	pressAt(m, 0, 0)
	got = pressAt(m, 0, 1) // q -> q
	if got == nil || *got != "q" {
		t.Fatalf("expected q after leaving numbers mode, got %v", got)
	}
}

func TestSimpleNeverEmitsControlCodes(t *testing.T) {
	// The server rejects SpecialKeyShift/SpecialKeyNumbers; the simple model
	// must consume those toggles locally and never send them.
	m := NewKeyboardModelSimple(simpleTestLayout)
	for _, p := range [][2]int{{0, 0}, {1, 0}} { // 123, SFT
		if got := pressAt(m, p[0], p[1]); got != nil {
			if r := []rune(*got); len(r) == 1 &&
				(r[0] == protocol.SpecialKeyShift || r[0] == protocol.SpecialKeyNumbers) {
				t.Fatalf("model emitted a control code the server rejects: %q", *got)
			}
		}
	}
}

func TestSimpleSpecialKeysStillPass(t *testing.T) {
	m := NewKeyboardModelSimple(simpleTestLayout)
	got := pressAt(m, 1, 10) // SPC
	if got == nil || []rune(*got)[0] != protocol.SpecialKeySpace {
		t.Fatalf("expected space, got %v", got)
	}
	got = pressAt(m, 2, 8) // ENT
	if got == nil || []rune(*got)[0] != protocol.SpecialKeyEnter {
		t.Fatalf("expected enter, got %v", got)
	}
}

func TestSimpleDPadStaleTickGuard(t *testing.T) {
	m := NewKeyboardModelSimple(simpleTestLayout)
	// Press right (gen bumps to 1), then release (gen bumps to 2).
	m.HandleKey(protocol.KeyDPadRight, true)
	startCol := m.selectedCol
	staleGen := m.holdGen // == 1, captured by the initial tick
	m.HandleKey(protocol.KeyDPadRight, false)

	// A stale tick from the first press fires after release; it must be ignored.
	m.HandleMsg(DPadHoldTickMsg{Key: protocol.KeyDPadRight, Gen: staleGen})
	if m.selectedCol != startCol {
		t.Fatalf("stale tick moved cursor: start=%d now=%d", startCol, m.selectedCol)
	}
}
