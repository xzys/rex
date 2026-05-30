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

func TestSimpleSymbolsPage2(t *testing.T) {
	m := NewKeyboardModelSimple(simpleTestLayout)
	pressAt(m, 0, 0) // 123 -> num1
	if got := pressAt(m, 0, 1); got == nil || *got != "1" { // q -> 1
		t.Fatalf("expected 1 on page1, got %v", got)
	}
	// SFT on the numbers page flips to the extended symbols page.
	if got := pressAt(m, 1, 0); got != nil {
		t.Fatalf("SFT page toggle should emit nil, got %v", *got)
	}
	if got := pressAt(m, 0, 1); got == nil || *got != "`" { // q -> `
		t.Fatalf("expected backtick on page2, got %v", got)
	}
	if got := pressAt(m, 1, 1); got == nil || *got != "|" { // a -> |
		t.Fatalf("expected pipe on page2, got %v", got)
	}
	// Inert key on page2 emits nothing.
	if got := pressAt(m, 1, 7); got != nil { // j has no page2 mapping
		t.Fatalf("inert page2 key should emit nil, got %v", *got)
	}
	// SFT flips back to page1.
	pressAt(m, 1, 0)
	if got := pressAt(m, 0, 1); got == nil || *got != "1" { // q -> 1
		t.Fatalf("expected 1 back on page1, got %v", got)
	}
	// 123 from a numbers page returns to letters.
	pressAt(m, 0, 0)
	if got := pressAt(m, 0, 1); got == nil || *got != "q" { // q -> q
		t.Fatalf("expected q back in letters, got %v", got)
	}
}

// TestSimpleLayersOn4x8 proves the layer behavior is layout-agnostic: the same
// shift/numbers/symbols logic works on the 4-row 4x8 grid, where the toggle
// keys and several letters sit in different positions than the 3-row layout.
func TestSimpleLayersOn4x8(t *testing.T) {
	find := func(layout [][]string, label string) (int, int) {
		for i, row := range layout {
			for j, k := range row {
				if k == label {
					return i, j
				}
			}
		}
		t.Fatalf("label %q not in 4x8 layout", label)
		return 0, 0
	}
	m := NewKeyboardModelSimple(keyboardLayout4x8.keys)
	press := func(label string) *string {
		r, c := find(keyboardLayout4x8.keys, label)
		return pressAt(m, r, c)
	}

	if got := press("a"); got == nil || *got != "a" {
		t.Fatalf("letters: expected a, got %v", got)
	}
	press("SFT") // sticky caps
	if got := press("a"); got == nil || *got != "A" {
		t.Fatalf("shift: expected A, got %v", got)
	}
	press("SFT")  // caps off
	press("123")  // -> page1
	if got := press("t"); got == nil || *got != "5" { // t is on the bottom row here
		t.Fatalf("page1: expected 5, got %v", got)
	}
	press("SFT") // page1 -> page2
	if got := press("t"); got == nil || *got != "=" {
		t.Fatalf("page2: expected =, got %v", got)
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
