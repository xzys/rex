package keyboard

import (
	"testing"

	cfg "github.com/xrfr/t-rex/v2/config"
)

func TestNewKeyboardUsesConfiguredLayout(t *testing.T) {
	k := NewKeyboard(cfg.KeyboardModeSimple, cfg.DefaultDictWords, true, false, cfg.STTModeSherpa, false, false, cfg.KeyboardViewGrid, cfg.KeyboardLayout3Row)

	if got, want := len(k.keyboard), 3; got != want {
		t.Fatalf("row count: got %d, want %d", got, want)
	}
	if got, want := len(k.keyboard[0]), 12; got != want {
		t.Fatalf("first row width: got %d, want %d", got, want)
	}
	if k.handCols != 0 {
		t.Fatalf("3-row layout should not use a center hand gap, got handCols=%d", k.handCols)
	}
}
