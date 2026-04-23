package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xrfr/t-rex/v2/protocol"
)

func TestGlobalOperatorFallbackDPadMappings(t *testing.T) {
	logPath := setupFakeTmux(t)
	op := NewGlobalOperator(&Server{}, nil)

	tests := []struct {
		name     string
		key      int
		expected string
	}{
		{name: "up sends up arrow", key: protocol.KeyDPadUp, expected: "send-keys -t 0 Up"},
		{name: "down sends down arrow", key: protocol.KeyDPadDown, expected: "send-keys -t 0 Down"},
		{name: "left scrolls up", key: protocol.KeyDPadLeft, expected: "copy-mode -eu -t 0"},
		{name: "right scrolls down", key: protocol.KeyDPadRight, expected: "send-keys -t 0 -X page-down"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetFakeTmuxLog(t, logPath)

			if err := op.HandleInput(InputEvent{Key: tc.key, IsDown: true}); err != nil {
				t.Fatalf("HandleInput: %v", err)
			}

			got := readFakeTmuxLog(t, logPath)
			if got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestClaudeOperatorDPadMappings(t *testing.T) {
	logPath := setupFakeTmux(t)
	op := NewClaudeOperator()

	tests := []struct {
		name     string
		key      int
		expected string
	}{
		{name: "up sends up arrow", key: protocol.KeyDPadUp, expected: "send-keys -t 0 Up"},
		{name: "down sends down arrow", key: protocol.KeyDPadDown, expected: "send-keys -t 0 Down"},
		{name: "left scrolls up", key: protocol.KeyDPadLeft, expected: "copy-mode -eu -t 0"},
		{name: "right scrolls down", key: protocol.KeyDPadRight, expected: "send-keys -t 0 -X page-down"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetFakeTmuxLog(t, logPath)

			handled, err := op.HandleInput(tc.key, true)
			if err != nil {
				t.Fatalf("HandleInput: %v", err)
			}
			if !handled {
				t.Fatalf("expected key to be handled")
			}

			got := readFakeTmuxLog(t, logPath)
			if got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func setupFakeTmux(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "tmux.log")
	tmuxPath := filepath.Join(tmpDir, "tmux")

	script := `#!/bin/sh
if [ "$1" = "display-message" ]; then
  echo "shell"
  exit 0
fi
printf '%s\n' "$*" >> "$REX_TEST_TMUX_LOG"
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)
	t.Setenv("REX_TEST_TMUX_LOG", logPath)
	return logPath
}

func resetFakeTmuxLog(t *testing.T, logPath string) {
	t.Helper()
	if err := os.WriteFile(logPath, nil, 0644); err != nil {
		t.Fatalf("reset log: %v", err)
	}
}

func readFakeTmuxLog(t *testing.T, logPath string) string {
	t.Helper()
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	return strings.Join(strings.Fields(strings.TrimSpace(string(data))), " ")
}
