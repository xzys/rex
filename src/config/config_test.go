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

func testWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
}
