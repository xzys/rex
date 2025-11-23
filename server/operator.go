package server

import (
	"fmt"
	"log/slog"

	"github.com/xzys/TR3X/v2/protocol"
)

func (s *Server) handleKey(key int) error {
	// TODO add more modes here based on active app

	var err error
	switch s.mode {
	case inputModeGlobal:
		switch key {
		case protocol.KeyR2:
			if err = s.openKeyboardPane(); err == nil {
				s.mode = inputModeKeyboard
			}
		default:
			slog.Warn("key not mapped, ignoring", "key", key)
		}
	case inputModeKeyboard:
		switch key {
		case protocol.KeyR2:
			if err = s.closeKeyboardPane(); err == nil {
				s.mode = inputModeGlobal
			}
		default:
			// send key input message to keyboard
			err = s.sendKeyInputMessage(key)
		}
	}
	return err
}

func (s *Server) sendKeyInputMessage(key int) error {
	if s.encoder == nil {
		slog.Error("trying to send key input w/o encoder")
		return nil
	}

	msg := protocol.KeyInputMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput},
		KeyInput:    key,
	}
	if err := s.encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to encode key input: %w", err)
	}
	slog.Debug("sent key input", "key", key)
	return nil
}
