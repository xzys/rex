package utils

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lmittmann/tint"
)

func setupLogger() {
	logFileName := "/tmp/tr3x.log"

	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
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
