package server

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/xzys/TR3X/v2/protocol"
)

func (s *Server) HandleKey(key int) error {
	// global use
	if s.keyboardConn == nil {
		slog.Warn("no connection", "key", key)
		return nil
	}

	// when keyboard is active
	msg := protocol.KeyInputMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput},
		KeyInput:    key,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if _, err := (*s.keyboardConn).Write(data); err != nil {
		return fmt.Errorf("failed to send key input: %w", err)
	}
	return nil
}
