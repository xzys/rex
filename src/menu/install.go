package menu

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	cfg "github.com/xrfr/t-rex/v2/config"
)

//go:embed install/*.sh
var embeddedInstallScripts embed.FS

// isAppInstalled reports whether the first token of app.Command resolves on PATH.
// Pseudo-commands like __tmux__ / __new_window__ are always considered installed.
func isAppInstalled(app cfg.App) bool {
	cmd := strings.TrimSpace(app.Command)
	if cmd == "" || strings.HasPrefix(cmd, "__") {
		return true
	}
	bin := strings.Fields(cmd)[0]
	_, err := exec.LookPath(bin)
	return err == nil
}

// installScript returns the embedded install script for the named app, or
// (nil, false) if no script ships for that name.
func installScript(name string) ([]byte, bool) {
	data, err := embeddedInstallScripts.ReadFile("install/" + name + ".sh")
	if err != nil {
		return nil, false
	}
	return data, true
}

// installScriptNames returns the set of app names that have an embedded
// install script. Used at startup for logging / sanity.
func installScriptNames() []string {
	var names []string
	_ = fs.WalkDir(embeddedInstallScripts, "install", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".sh") {
			return err
		}
		names = append(names, strings.TrimSuffix(filepath.Base(path), ".sh"))
		return nil
	})
	return names
}

// writeInstallScript drops the embedded script for `name` into a temp file
// and returns its path. Caller is responsible for removing it.
func writeInstallScript(name string) (string, error) {
	data, ok := installScript(name)
	if !ok {
		return "", fmt.Errorf("no install script for %q", name)
	}
	path := filepath.Join(os.TempDir(), "rex-install-"+name+".sh")
	if err := os.WriteFile(path, data, 0755); err != nil {
		return "", fmt.Errorf("write install script: %w", err)
	}
	return path, nil
}
