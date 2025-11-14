package protocol

import (
	"encoding/json"
	"fmt"
)

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
func ParseMessage(data []byte) (interface{}, error) {
	var base map[string]interface{}
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}

	reqType, ok := base["type"].(string)
	if !ok {
		return nil, json.Unmarshal(data, &PressKeyMessage{})
	}

	var req any
	switch reqType {
	case MessageTypeKeyInput:
		req = new(KeyInputMessage)
	case MessageTypeKeyPress:
		req = new(PressKeyMessage)
	default:
		return nil, fmt.Errorf("unknown message type: %s", reqType)
	}
	if err := json.Unmarshal(data, req); err != nil {
		return nil, err
	}
	return req, nil
}
