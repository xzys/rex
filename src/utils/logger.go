package utils

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/lmittmann/tint"
)

func setupLogger() {
	logFileName := "/tmp/rex.log"

	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	// On Brick, rex runs both as root (wifi capture in start-rex.sh) and as
	// the unprivileged 'rex' user inside the Debian chroot. The umask drops
	// the file to 0600, so whichever user creates it first locks out the
	// other. Force 0666 here so subsequent processes can always append.
	_ = os.Chmod(logFileName, 0666)

	// Mirror panic/fatal tracebacks (including CGo-triggered SIGSEGV fatals,
	// e.g. inside sherpa-onnx) into the log file. Without this they go only to
	// the process's stderr, which for the keyboard is a transient tmux pane
	// that vanishes on crash — so crashes left no trace. Normal stderr is
	// untouched, so CLI usage/error messages still reach the terminal.
	if err := debug.SetCrashOutput(logFile, debug.CrashOptions{}); err != nil {
		slog.Warn("failed to set crash output", "err", err)
	}

	slog.SetDefault(slog.New(tint.NewHandler(logFile, &tint.Options{
		Level:     slog.LevelDebug,
		AddSource: true,
		// replace source path with relative path
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				// Get the working directory
				wd, err := os.Getwd()
				if err == nil {
					// Convert to relative path
					if rel, err := filepath.Rel(wd, source.File); err == nil {
						source.File = rel
					}
				}
				// Also remove any leading "../" for cleaner paths
				source.File = strings.TrimPrefix(source.File, "../")
			}
			return a
		},
	})))
}

func init() {
	setupLogger()
}
