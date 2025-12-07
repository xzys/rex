package server

import (
	"fmt"
	"log/slog"
	"os/exec"
)

func execute(arg0 string, args ...string) error {
	cmd := exec.Command(arg0, args...)
	slog.Debug("executing command", "command", cmd.String())

	if output, err := cmd.CombinedOutput(); err != nil {
		slog.Error("command failed", "command", cmd.String(), "err", err, "output", string(output))
		return fmt.Errorf("failed to execute command: %w", err)
	}
	return nil
}
