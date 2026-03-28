package protocol

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	// InputSocketPath is for server -> keyboard messages (KeyInput)
	defaultInputSocketPath = "/tmp/rex-input.sock"
	// OutputSocketPath is for keyboard -> server messages (KeyPress)
	defaultOutputSocketPath = "/tmp/rex-output.sock"
	envInputSocketPath      = "REX_INPUT_SOCKET"
	envOutputSocketPath     = "REX_OUTPUT_SOCKET"
)

func InputSocketPath() string {
	if path := os.Getenv(envInputSocketPath); path != "" {
		return path
	}
	return defaultInputSocketPath
}

func OutputSocketPath() string {
	if path := os.Getenv(envOutputSocketPath); path != "" {
		return path
	}
	return defaultOutputSocketPath
}

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
	SpecialKeyMic     = '\x05' // Control-E — enters voice mode
	SpecialKeyEnter   = '\n'
)

// requests
const (
	MessageTypeKeyInput    = "key_input"
	MessageTypeKeyPress    = "key_press"
	MessageTypePressString = "press_string"
)

type BaseMessage struct {
	Type string `json:"type"`
}

// sent from server to keyboard client with raw device input for translation
type KeyInputMessage struct {
	BaseMessage
	KeyInput  int  `json:"key_input"` // raw key code from the input device
	IsKeyDown bool `json:"is_key_down"`
}

// response from keyboard client with the translated virtual key
type PressKeyMessage struct {
	BaseMessage
	KeyPress string `json:"key_press"` // virtual key character to press
}

// response from keyboard client with a string to type (e.g. autocomplete)
type PressStringMessage struct {
	BaseMessage
	Value string `json:"value"` // string to type character by character
}

// ParseMessage decodes a JSON message from the decoder
func ParseMessage(decoder *json.Decoder) (any, error) {
	// Decode into a map to check the type field
	var rawMap map[string]interface{}
	if err := decoder.Decode(&rawMap); err != nil {
		return nil, err
	}

	// Get the message type
	msgType, ok := rawMap["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid type field")
	}

	// Marshal back to bytes for final unmarshal into typed struct
	data, err := json.Marshal(rawMap)
	if err != nil {
		return nil, err
	}

	// Unmarshal into the appropriate message type
	switch msgType {
	case MessageTypeKeyInput:
		var msg KeyInputMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeKeyPress:
		var msg PressKeyMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypePressString:
		var msg PressStringMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	default:
		return nil, fmt.Errorf("unknown message type: %s", msgType)
	}
}
