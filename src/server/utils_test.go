package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTryExecuteIgnoreMessages(t *testing.T) {
	tmpDir := t.TempDir()
	cmdPath := filepath.Join(tmpDir, "fakecmd")
	script := `#!/bin/sh
case "$1" in
  ignored)
    echo "not in a mode" >&2
    exit 1
    ;;
  other)
    echo "unexpected failure" >&2
    exit 1
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(cmdPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake command: %v", err)
	}

	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+oldPath)

	if err := tryExecuteIgnoreMessages([]string{"not in a mode"}, "fakecmd", "ignored"); err != nil {
		t.Fatalf("ignored message returned error: %v", err)
	}

	if err := tryExecuteIgnoreMessages([]string{"not in a mode"}, "fakecmd", "other"); err == nil {
		t.Fatal("unexpected success for non-ignored failure")
	}
}
