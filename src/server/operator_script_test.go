package server

import (
	"testing"

	"github.com/xrfr/t-rex/v2/protocol"
)

func TestScriptOperatorNcmpcpp(t *testing.T) {
	script := `
let screen = 0;
function handleKey(key) {
  switch (key) {
    case KEY_DPAD_RIGHT: screen = (screen + 1) % 8; return String(screen + 1);
    case KEY_DPAD_LEFT:  screen = (screen - 1 + 8) % 8; return String(screen + 1);
    case KEY_B: return "p";
    case KEY_X: return ">";
    case KEY_Y: return "<";
  }
  return null;
}
`
	op, err := NewScriptOperator("ncmpcpp", script)
	if err != nil {
		t.Fatalf("NewScriptOperator: %v", err)
	}
	defer op.Close()

	cases := []struct {
		name    string
		key     int
		wantKey string
		wantOK  bool
	}{
		{"right cycles 0->1", protocol.KeyDPadRight, "2", true},
		{"right cycles 1->2", protocol.KeyDPadRight, "3", true},
		{"left cycles 2->1", protocol.KeyDPadLeft, "2", true},
		{"left wraps 1->0 wraps to 8", protocol.KeyDPadLeft, "1", true},
		{"left wraps 0 to 7 -> 8", protocol.KeyDPadLeft, "8", true},
		{"B -> p", protocol.KeyB, "p", true},
		{"X -> >", protocol.KeyX, ">", true},
		{"Y -> <", protocol.KeyY, "<", true},
		{"A falls through", protocol.KeyA, "", false},
		{"Select falls through", protocol.KeySelect, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotKey, gotOK := op.evalKey(tc.key)
			if gotKey != tc.wantKey || gotOK != tc.wantOK {
				t.Fatalf("evalKey(%d) = (%q, %v), want (%q, %v)", tc.key, gotKey, gotOK, tc.wantKey, tc.wantOK)
			}
		})
	}
}

func TestScriptOperatorMissingHandleKey(t *testing.T) {
	_, err := NewScriptOperator("bad", `const x = 1;`)
	if err == nil {
		t.Fatal("expected error for missing handleKey; got nil")
	}
}

func TestScriptOperatorThrowsIsUnhandled(t *testing.T) {
	op, err := NewScriptOperator("throwy", `function handleKey(k) { throw new Error("boom"); }`)
	if err != nil {
		t.Fatalf("NewScriptOperator: %v", err)
	}
	defer op.Close()

	handled, err := op.HandleInput(protocol.KeyA, true)
	if err != nil {
		t.Fatalf("HandleInput returned error: %v", err)
	}
	if handled {
		t.Fatalf("expected unhandled when plugin throws")
	}
}
