package server

import (
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
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

// tmuxOutput runs a tmux command and returns its trimmed stdout.
func tmuxOutput(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	slog.Debug("executing command", "command", cmd.String())
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func tryExecuteIgnoreMessages(ignoreMessages []string, arg0 string, args ...string) error {
	cmd := exec.Command(arg0, args...)
	slog.Debug("executing command", "command", cmd.String())

	if output, err := cmd.CombinedOutput(); err != nil {
		outputStr := string(output)
		for _, ignoreMsg := range ignoreMessages {
			if strings.Contains(outputStr, ignoreMsg) {
				slog.Warn("command failed but ignoring", "command", cmd.String(), "reason", ignoreMsg)
				return nil
			}
		}
		slog.Error("command failed", "command", cmd.String(), "err", err, "output", outputStr)
		return fmt.Errorf("failed to execute command: %w", err)
	}
	return nil
}
