package server

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/xrfr/t-rex/v2/protocol"
)

type StdinInputDevice struct {
	reader io.Reader
}

func NewStdinInputDevice(reader io.Reader) *StdinInputDevice {
	return &StdinInputDevice{reader: reader}
}

func (d *StdinInputDevice) Initialize(handler func(InputEvent) error) error {
	go runStdinInputLoop(d.reader, handler)
	return nil
}

func runStdinInputLoop(reader io.Reader, handler func(InputEvent) error) {
	scanner := bufio.NewScanner(reader)
	slog.Info("listening for stdin input events", "format", "down|up <key>")
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			slog.Warn("invalid stdin input", "line", line)
			continue
		}

	isDown, err := parseInputAction(fields[0])
		if err != nil {
			slog.Warn("invalid stdin action", "action", fields[0], "err", err)
			continue
		}

		key, err := parseInputKey(fields[1])
		if err != nil {
			slog.Warn("invalid stdin key", "key", fields[1], "err", err)
			continue
		}

	if err := handler(InputEvent{Key: key, IsDown: isDown}); err != nil {
			slog.Error("stdin input handler error", "err", err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("stdin input scan error", "err", err)
	}
}

func parseInputAction(token string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "down":
		return true, nil
	case "up":
		return false, nil
	default:
		return false, fmt.Errorf("unknown action: %s", token)
	}
}

func parseInputKey(token string) (int, error) {
	normalized := normalizeKeyToken(token)
	if key, ok := stdinKeyMap[normalized]; ok {
		return key, nil
	}
	return 0, fmt.Errorf("unknown key: %s", token)
}

func normalizeKeyToken(token string) string {
	normalized := strings.ToLower(strings.TrimSpace(token))
	normalized = strings.TrimPrefix(normalized, "key")
	normalized = strings.TrimPrefix(normalized, "_")
	normalized = strings.ReplaceAll(normalized, "-", "")
	return normalized
}

var stdinKeyMap = map[string]int{
	"a":        protocol.KeyA,
	"b":        protocol.KeyB,
	"x":        protocol.KeyX,
	"y":        protocol.KeyY,
	"dpadup":   protocol.KeyDPadUp,
	"dpaddown": protocol.KeyDPadDown,
	"dpadleft": protocol.KeyDPadLeft,
	"dpadright": protocol.KeyDPadRight,
	"l1":       protocol.KeyL1,
	"r1":       protocol.KeyR1,
	"l2":       protocol.KeyL2,
	"r2":       protocol.KeyR2,
	"l3":       protocol.KeyL3,
	"r3":       protocol.KeyR3,
	"select":   protocol.KeySelect,
	"start":    protocol.KeyStart,
	"fn":       protocol.KeyFn,
}
