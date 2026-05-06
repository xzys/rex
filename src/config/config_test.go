package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaultsToCompactKeyboard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	cfg, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatalf("load missing config: %v", err)
	}

	if !cfg.KeyboardCompact {
		t.Fatalf("expected missing config to default keyboard_compact to true")
	}
	if cfg.KeyboardLayout != KeyboardLayout4x8 {
		t.Fatalf("expected missing config to default keyboard_layout to %q, got %q", KeyboardLayout4x8, cfg.KeyboardLayout)
	}
	if cfg.KeyboardView != KeyboardViewGrid {
		t.Fatalf("expected missing config to default keyboard_view to %q, got %q", KeyboardViewGrid, cfg.KeyboardView)
	}
}

func TestLoadConfigMissingCompactDefaultsTrue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	testWriteFile(t, path, "keyboard_mode: front-chorded\n")

	cfg, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if !cfg.KeyboardCompact {
		t.Fatalf("expected omitted keyboard_compact to default to true")
	}
	if cfg.KeyboardLayout != KeyboardLayout4x8 {
		t.Fatalf("expected omitted keyboard_layout to default to %q, got %q", KeyboardLayout4x8, cfg.KeyboardLayout)
	}
	if cfg.KeyboardView != KeyboardViewGrid {
		t.Fatalf("expected omitted keyboard_view to default to %q, got %q", KeyboardViewGrid, cfg.KeyboardView)
	}
}

func TestLoadConfigExplicitCompactFalse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	testWriteFile(t, path, "keyboard_compact: false\n")

	cfg, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.KeyboardCompact {
		t.Fatalf("expected explicit keyboard_compact: false to be preserved")
	}
}

func TestLoadConfigExplicitKeyboardView(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	testWriteFile(t, path, "keyboard_view: fan\n")

	cfg, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.KeyboardView != KeyboardViewFan {
		t.Fatalf("keyboard_view: got %q, want %q", cfg.KeyboardView, KeyboardViewFan)
	}
}

func TestLoadConfigExplicitKeyboardLayout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	testWriteFile(t, path, "keyboard_layout: 3-row\n")

	cfg, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.KeyboardLayout != KeyboardLayout3Row {
		t.Fatalf("keyboard_layout: got %q, want %q", cfg.KeyboardLayout, KeyboardLayout3Row)
	}
}

func TestKeyboardPaneHeightForLayout(t *testing.T) {
	if got := KeyboardPaneHeightForLayout(true, false, KeyboardLayout4x8); got != 8 {
		t.Fatalf("4x8 compact height: got %d, want 8", got)
	}
	if got := KeyboardPaneHeightForLayout(true, false, KeyboardLayout3Row); got != 6 {
		t.Fatalf("3-row compact height: got %d, want 6", got)
	}
	if got := KeyboardPaneHeightForLayout(false, true, KeyboardLayout3Row); got != 10 {
		t.Fatalf("3-row normal autocomplete height: got %d, want 10", got)
	}
}

func testWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
}
