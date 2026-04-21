package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xrfr/t-rex/v2/protocol"
)

func TestGlobalOperatorFallbackDPadMappings(t *testing.T) {
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

	op := NewGlobalOperator(&Server{}, nil)

	tests := []struct {
		name     string
		key      int
		expected string
	}{
		{name: "up enters copy mode", key: protocol.KeyDPadUp, expected: "copy-mode -eu -t 0"},
		{name: "down pages down", key: protocol.KeyDPadDown, expected: "send-keys -t 0 -X page-down"},
		{name: "left sends up arrow", key: protocol.KeyDPadLeft, expected: "send-keys -t 0 Up"},
		{name: "right sends down arrow", key: protocol.KeyDPadRight, expected: "send-keys -t 0 Down"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(logPath, nil, 0644); err != nil {
				t.Fatalf("reset log: %v", err)
			}

			if err := op.HandleInput(InputEvent{Key: tc.key, IsDown: true}); err != nil {
				t.Fatalf("HandleInput: %v", err)
			}

			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatalf("read log: %v", err)
			}
			lines := strings.Fields(strings.TrimSpace(string(data)))
			got := strings.Join(lines, " ")
			if got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}
