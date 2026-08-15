package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		{name: "up sends up arrow", key: protocol.KeyDPadUp, expected: "send-keys Up"},
		{name: "down sends down arrow", key: protocol.KeyDPadDown, expected: "send-keys Down"},
		{name: "left sends left arrow", key: protocol.KeyDPadLeft, expected: "send-keys Left"},
		{name: "right sends right arrow", key: protocol.KeyDPadRight, expected: "send-keys Right"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetFakeTmuxLog(t, logPath)

			if err := op.HandleInput(InputEvent{Key: tc.key, IsDown: true}); err != nil {
				t.Fatalf("HandleInput: %v", err)
			}
			// release, so the key repeater doesn't keep firing into later tests
			if err := op.HandleInput(InputEvent{Key: tc.key, IsDown: false}); err != nil {
				t.Fatalf("HandleInput up: %v", err)
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
		{name: "up sends up arrow", key: protocol.KeyDPadUp, expected: "send-keys Up"},
		{name: "down sends down arrow", key: protocol.KeyDPadDown, expected: "send-keys Down"},
		{name: "left sends left arrow", key: protocol.KeyDPadLeft, expected: "send-keys Left"},
		{name: "right sends right arrow", key: protocol.KeyDPadRight, expected: "send-keys Right"},
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
			// release, so the key repeater doesn't keep firing into later tests
			if _, err := op.HandleInput(tc.key, false); err != nil {
				t.Fatalf("HandleInput up: %v", err)
			}

			got := readFakeTmuxLog(t, logPath)
			if got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

// In copy-mode the D-pad must scroll the viewport (trackpad-style) instead of
// walking the copy cursor one line at a time. Left/right stay literal arrows.
func TestDPadScrollsInCopyMode(t *testing.T) {
	tests := []struct {
		name     string
		key      int
		expected string
	}{
		{name: "up scrolls viewport up", key: protocol.KeyDPadUp, expected: "send-keys -X scroll-up"},
		{name: "down scrolls viewport down", key: protocol.KeyDPadDown, expected: "send-keys -X scroll-down"},
		{name: "left still sends left arrow", key: protocol.KeyDPadLeft, expected: "send-keys Left"},
		{name: "right still sends right arrow", key: protocol.KeyDPadRight, expected: "send-keys Right"},
	}

	t.Run("global fallback", func(t *testing.T) {
		logPath := setupCopyModeFakeTmux(t)
		op := NewGlobalOperator(&Server{}, nil)

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				resetFakeTmuxLog(t, logPath)

				if err := op.HandleInput(InputEvent{Key: tc.key, IsDown: true}); err != nil {
					t.Fatalf("HandleInput: %v", err)
				}
				if err := op.HandleInput(InputEvent{Key: tc.key, IsDown: false}); err != nil {
					t.Fatalf("HandleInput up: %v", err)
				}

				if got := readFakeTmuxLog(t, logPath); got != tc.expected {
					t.Fatalf("got %q, want %q", got, tc.expected)
				}
			})
		}
	})

	// The claude window has its own operator, which claims the D-pad before the
	// global fallback ever runs — it needs the same copy-mode awareness.
	t.Run("claude operator", func(t *testing.T) {
		logPath := setupCopyModeFakeTmux(t)
		op := NewClaudeOperator()

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
				if _, err := op.HandleInput(tc.key, false); err != nil {
					t.Fatalf("HandleInput up: %v", err)
				}

				if got := readFakeTmuxLog(t, logPath); got != tc.expected {
					t.Fatalf("got %q, want %q", got, tc.expected)
				}
			})
		}
	})
}

// TestCyclePaneSkipsKeyboardPane: focus cycling must never land on the
// keyboard pane — it holds no focus by design.
func TestCyclePaneSkipsKeyboardPane(t *testing.T) {
	logPath := setupNavFakeTmux(t)
	op := NewGlobalOperator(&Server{}, nil)
	op.keyboardActive = true
	op.keyboardPane = "%9"

	tests := []struct {
		name     string
		step     int
		expected string
	}{
		// panes are [%0 (active), %9 (keyboard), %2] — both directions skip %9
		{name: "forward skips keyboard", step: 1, expected: "select-pane -t %2"},
		{name: "backward skips keyboard", step: -1, expected: "select-pane -t %2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetFakeTmuxLog(t, logPath)
			if err := op.cyclePane(tc.step); err != nil {
				t.Fatalf("cyclePane: %v", err)
			}
			if got := readFakeTmuxLog(t, logPath); got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

// TestWindowNavMovesKeyboardPane: switching windows re-attaches the live
// keyboard pane instead of killing it.
func TestWindowNavMovesKeyboardPane(t *testing.T) {
	logPath := setupNavFakeTmux(t)
	op := NewGlobalOperator(&Server{}, nil)
	op.keyboardActive = true
	op.keyboardPane = "%9"

	// R1 press + release = next window
	if err := op.HandleInput(InputEvent{Key: protocol.KeyR1, IsDown: true}); err != nil {
		t.Fatalf("HandleInput down: %v", err)
	}
	if err := op.HandleInput(InputEvent{Key: protocol.KeyR1, IsDown: false}); err != nil {
		t.Fatalf("HandleInput up: %v", err)
	}

	got := readFakeTmuxLog(t, logPath)
	want := "next-window join-pane -d -v -l 8 -s %9"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if !op.keyboardActive || op.keyboardPane != "%9" {
		t.Fatalf("keyboard should survive window nav (active=%v pane=%q)", op.keyboardActive, op.keyboardPane)
	}
}

// TestShoulderChordDoesNotSwitchWindow: after an L1+R1 pane-cycle chord,
// releasing the shoulders (either order) must not fire a window switch.
func TestShoulderChordDoesNotSwitchWindow(t *testing.T) {
	for _, releaseOrder := range []struct {
		name          string
		first, second int
	}{
		{name: "release R1 first", first: protocol.KeyR1, second: protocol.KeyL1},
		{name: "release L1 first", first: protocol.KeyL1, second: protocol.KeyR1},
	} {
		t.Run(releaseOrder.name, func(t *testing.T) {
			logPath := setupNavFakeTmux(t)
			op := NewGlobalOperator(&Server{}, nil)
			op.keyboardActive = true
			op.keyboardPane = "%9"

			press := func(key int, down bool) {
				t.Helper()
				if err := op.HandleInput(InputEvent{Key: key, IsDown: down}); err != nil {
					t.Fatalf("HandleInput(%d, %v): %v", key, down, err)
				}
			}
			press(protocol.KeyL1, true)
			time.Sleep(shoulderChordWindow + 50*time.Millisecond) // past the kill-pane chord window
			press(protocol.KeyR1, true)
			press(releaseOrder.first, false)
			press(releaseOrder.second, false)

			got := readFakeTmuxLog(t, logPath)
			want := "select-pane -t %2"
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestWindowNavWithoutKeyboard(t *testing.T) {
	logPath := setupNavFakeTmux(t)
	op := NewGlobalOperator(&Server{}, nil)

	if err := op.nextWindow(); err != nil {
		t.Fatalf("nextWindow: %v", err)
	}
	if got := readFakeTmuxLog(t, logPath); got != "next-window" {
		t.Fatalf("got %q, want %q", got, "next-window")
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

// setupCopyModeFakeTmux installs a fake tmux that reports the active pane as
// being in copy-mode, and logs every mutating command.
func setupCopyModeFakeTmux(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "tmux.log")
	tmuxPath := filepath.Join(tmpDir, "tmux")

	script := `#!/bin/sh
case "$*" in
  "display-message -p #{pane_in_mode}") echo "1"; exit 0;;
  display-message*) echo "shell"; exit 0;;
esac
printf '%s\n' "$*" >> "$REX_TEST_TMUX_LOG"
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)
	t.Setenv("REX_TEST_TMUX_LOG", logPath)
	if err := os.WriteFile(logPath, nil, 0644); err != nil {
		t.Fatalf("init log: %v", err)
	}
	return logPath
}

// setupNavFakeTmux installs a fake tmux that answers the pane/window queries
// used by keyboard-aware navigation (keyboard pane %9 in window @1, current
// window @2) and logs every mutating command.
func setupNavFakeTmux(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "tmux.log")
	tmuxPath := filepath.Join(tmpDir, "tmux")

	script := `#!/bin/sh
case "$*" in
  "list-panes -F #{pane_id} #{pane_active}")
    printf '%s\n' "%0 1" "%9 0" "%2 0"; exit 0;;
  "display-message -p -t %9 #{window_id}") echo "@1"; exit 0;;
  "display-message -p #{window_id}") echo "@2"; exit 0;;
  "display-message -p -t %9 #{pane_height}") echo "8"; exit 0;;
  display-message*) echo "shell"; exit 0;;
esac
printf '%s\n' "$*" >> "$REX_TEST_TMUX_LOG"
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)
	t.Setenv("REX_TEST_TMUX_LOG", logPath)
	if err := os.WriteFile(logPath, nil, 0644); err != nil {
		t.Fatalf("init log: %v", err)
	}
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
