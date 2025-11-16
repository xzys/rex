package server

import (
	"log/slog"

	"github.com/xzys/TR3X/v2/protocol"
)

// PressKey executes a key press
func pressKey(key string) error {
	if len(key) == 0 {
		return nil
	}

	char := rune(key[0])
	switch char {
	case protocol.SpecialKeySpace:
		slog.Info("sending space")
	case protocol.SpecialKeyShift:
		slog.Info("sending shift toggle")
	case protocol.SpecialKeyDelete:
		slog.Info("sending delete")
	case protocol.SpecialKeyNumbers:
		slog.Info("sending numbers toggle")
	case protocol.SpecialKeyClose:
		slog.Info("sending close")
	case protocol.SpecialKeyEnter:
		slog.Info("sending enter")
	default:
		slog.Info("sending key", "key", char)
	}
	return nil
}
