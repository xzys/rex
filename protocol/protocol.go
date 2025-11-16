package protocol

import (
	"encoding/json"
	"fmt"
)

const SocketPath = "/tmp/tr3x.sock"

const (
	KeyDPadUp = iota
	KeyDPadDown
	KeyDPadLeft
	KeyDPadRight
	KeyA
	KeyB
	KeyX
	KeyY
	KeyL1
	KeyR1
	KeyL2
	KeyR2
	KeySelect
	KeyStart
	KeyFn
)

// Special key constants (non-printable control codes to avoid collisions)
const (
	SpecialKeySpace   = ' '
	SpecialKeyShift   = '\x01' // Control-A
	SpecialKeyDelete  = '\x02' // Control-B
	SpecialKeyNumbers = '\x03' // Control-C
	SpecialKeyClose   = '\x04' // Control-D
	SpecialKeyEnter   = '\n'
)

// requests
const (
	MessageTypeKeyInput = "key_input"
	MessageTypeKeyPress = "key_press"
)

type BaseMessage struct {
	Type string `json:"type"`
}

// sent from server to keyboard client with raw device input for translation
type KeyInputMessage struct {
	BaseMessage
	KeyInput int `json:"key_input"` // raw key code from the input device
}

// response from keyboard client with the translated virtual key
type PressKeyMessage struct {
	BaseMessage
	KeyPress string `json:"key_press"` // virtual key character to press
}

// ParseMessage parses a JSON request message
func ParseMessage(data []byte) (any, error) {
	var base map[string]interface{}
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}

	reqType, ok := base["type"].(string)
	if !ok {
		return nil, json.Unmarshal(data, &PressKeyMessage{})
	}

	var msg any
	switch reqType {
	case MessageTypeKeyInput:
		msg = new(KeyInputMessage)
	case MessageTypeKeyPress:
		msg = new(PressKeyMessage)
	default:
		return nil, fmt.Errorf("unknown message type: %s", reqType)
	}
	if err := json.Unmarshal(data, msg); err != nil {
		return nil, err
	}
	return msg, nil
}
