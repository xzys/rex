package server

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

//go:embed plugins/*.js
var embeddedPlugins embed.FS

// LoadPlugins reads every .js file in ~/.config/rex/plugins/ and returns a
// map keyed by filename (minus .js) — matching tmux window names.
//
// If the directory is missing, falls back to plugins embedded in the binary.
// Individual plugins that fail to load are logged and skipped so a bad script
// can't brick the server.
func LoadPlugins() map[string]Operator {
	operators := map[string]Operator{}

	dir, err := pluginDir()
	if err != nil {
		slog.Warn("plugins: failed to resolve plugin dir, loading embedded defaults only", "err", err)
		loadEmbeddedPlugins(operators)
		return operators
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("plugins: failed to read plugin dir", "dir", dir, "err", err)
		}
		slog.Info("plugins: loading embedded defaults", "dir", dir)
		loadEmbeddedPlugins(operators)
		return operators
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".js") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".js")
		path := filepath.Join(dir, entry.Name())
		script, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("plugins: read failed", "plugin", name, "path", path, "err", err)
			continue
		}
		op, err := NewScriptOperator(name, string(script))
		if err != nil {
			slog.Warn("plugins: load failed", "plugin", name, "err", err)
			continue
		}
		operators[name] = op
		slog.Info("plugins: loaded", "plugin", name, "source", path)
	}

	// Fill in any embedded plugins not overridden by user files.
	loadEmbeddedPluginsIfMissing(operators)
	return operators
}

func loadEmbeddedPlugins(operators map[string]Operator) {
	loadEmbeddedPluginsIfMissing(operators)
}

func loadEmbeddedPluginsIfMissing(operators map[string]Operator) {
	err := fs.WalkDir(embeddedPlugins, "plugins", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".js") {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(path), ".js")
		if _, ok := operators[name]; ok {
			return nil
		}
		script, err := embeddedPlugins.ReadFile(path)
		if err != nil {
			slog.Warn("plugins: embedded read failed", "plugin", name, "err", err)
			return nil
		}
		op, err := NewScriptOperator(name, string(script))
		if err != nil {
			slog.Warn("plugins: embedded load failed", "plugin", name, "err", err)
			return nil
		}
		operators[name] = op
		slog.Info("plugins: loaded embedded", "plugin", name)
		return nil
	})
	if err != nil {
		slog.Warn("plugins: walk embedded failed", "err", err)
	}
}

func pluginDir() (string, error) {
	if env := os.Getenv("REX_PLUGIN_DIR"); env != "" {
		return env, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, ".config", "rex", "plugins"), nil
}
