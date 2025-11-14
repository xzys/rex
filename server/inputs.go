package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"

	"github.com/xzys/TR3X/v2/protocol"
)

type InputHandler struct {
	conn *net.Conn
}

func NewInputHandler() *InputHandler {
	return &InputHandler{}
}

func (h *InputHandler) SetKeyboardConn(conn *net.Conn) {
	h.conn = conn
}

func (h *InputHandler) HandleKey(key int) error {
	// global use
	if h.conn == nil {
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
	if _, err := (*h.conn).Write(data); err != nil {
		return fmt.Errorf("failed to send key input: %w", err)
	}
	return nil
}
