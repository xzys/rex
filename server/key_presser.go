package server

import (
	"log/slog"
)

// PressKey executes a key press
func PressKey(key string) error {
	if len(key) == 0 {
		return nil
	}

	char := rune(key[0])
	switch char {
	case ' ':
		slog.Info("sending space")
	case 'S':
		slog.Info("sending shift toggle")
	case 'D':
		slog.Info("sending delete")
	case '1':
		slog.Info("sending numbers toggle")
	case 'X':
		slog.Info("sending close")
	case 'E':
		slog.Info("sending enter")
	default:
		slog.Info("sending key", "key", char)
	}
	return nil
}
