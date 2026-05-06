package settings

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	cfg "github.com/xrfr/t-rex/v2/config"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	panic("unknown key: " + s)
}

func TestSettingsCycling(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("REX_CONFIG_PATH", path)

	start := cfg.Config{KeyboardMode: cfg.KeyboardModeChorded, STTMode: cfg.STTModeSherpa}
	if err := cfg.SaveConfig(start); err != nil {
		t.Fatalf("seed save: %v", err)
	}

	loaded, err := cfg.LoadConfig()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	var m tea.Model = NewModel(loaded)

	// Cursor starts on row 0 (keyboard mode). Cycle right once.
	m, _ = m.Update(keyMsg("right"))
	// Move down to keyboard layout row and cycle right (4x8 -> 3-row).
	m, _ = m.Update(keyMsg("down"))
	m, _ = m.Update(keyMsg("right"))
	// Move down to keyboard view row and cycle right (grid -> fan).
	m, _ = m.Update(keyMsg("down"))
	m, _ = m.Update(keyMsg("right"))
	// Move down to STT row, cycle right (sherpa -> parakeet).
	m, _ = m.Update(keyMsg("down"))
	m, _ = m.Update(keyMsg("right"))
	// Cycle right again (parakeet -> companion).
	m, _ = m.Update(keyMsg("right"))

	got, err := cfg.LoadConfig()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.STTMode != cfg.STTModeCompanion {
		t.Errorf("stt_mode: got %q, want %q", got.STTMode, cfg.STTModeCompanion)
	}
	if got.KeyboardMode == cfg.KeyboardModeChorded {
		t.Errorf("keyboard_mode did not advance from initial %q", got.KeyboardMode)
	}
	if got.KeyboardLayout != cfg.KeyboardLayout3Row {
		t.Errorf("keyboard_layout: got %q, want %q", got.KeyboardLayout, cfg.KeyboardLayout3Row)
	}
	if got.KeyboardView != cfg.KeyboardViewFan {
		t.Errorf("keyboard_view: got %q, want %q", got.KeyboardView, cfg.KeyboardViewFan)
	}

	// Sanity-check persistence: file on disk matches.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !contains(string(raw), "stt_mode: companion") {
		t.Errorf("stt_mode not written to file: %s", raw)
	}
	if !contains(string(raw), "keyboard_layout: 3-row") {
		t.Errorf("keyboard_layout not written to file: %s", raw)
	}
	if !contains(string(raw), "keyboard_view: fan") {
		t.Errorf("keyboard_view not written to file: %s", raw)
	}
}

func TestSettingsCursorBounds(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REX_CONFIG_PATH", filepath.Join(dir, "config.yaml"))

	m := NewModel(cfg.Config{})

	// Cursor starts at 0; up should keep it at 0.
	var model tea.Model = m
	model, _ = model.Update(keyMsg("up"))
	if m.cursor != 0 {
		t.Errorf("up at top: cursor=%d, want 0", m.cursor)
	}

	// Down should advance to rowSTTMode, further down should stop at rowCount-1.
	for i := 0; i < 5; i++ {
		model, _ = model.Update(keyMsg("down"))
	}
	if m.cursor != rowCount-1 {
		t.Errorf("down past end: cursor=%d, want %d", m.cursor, rowCount-1)
	}
	_ = model
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
