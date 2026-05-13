package server

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

const (
	defaultControlSocketPath = "/tmp/rex-control.sock"
	envControlSocketPath     = "REX_CONTROL_SOCKET"
)

func ControlSocketPath() string {
	if path := os.Getenv(envControlSocketPath); path != "" {
		return path
	}
	return defaultControlSocketPath
}

const (
	MessageTypeDisable      = "disable"
	MessageTypeInjectInput  = "inject_input"
	MessageTypeOpenKeyboard = "open_keyboard"
)

type DisableMessage struct {
	Type    string `json:"type"`
	Disable bool   `json:"disable"`
}

type InjectInputMessage struct {
	Type   string `json:"type"`
	Key    int    `json:"key"`
	IsDown bool   `json:"is_down"`
}

type OpenKeyboardMessage struct {
	Type  string `json:"type"`
	Voice bool   `json:"voice"`
}

func ParseControlMessage(decoder *json.Decoder) (any, error) {
	var rawMap map[string]interface{}
	if err := decoder.Decode(&rawMap); err != nil {
		return nil, err
	}

	msgType, ok := rawMap["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid type field")
	}

	data, err := json.Marshal(rawMap)
	if err != nil {
		return nil, err
	}

	switch msgType {
	case MessageTypeDisable:
		var msg DisableMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeInjectInput:
		var msg InjectInputMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case MessageTypeOpenKeyboard:
		var msg OpenKeyboardMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	default:
		return nil, fmt.Errorf("unknown message type: %s", msgType)
	}
}

// SendOpenKeyboard asks the running server to open the keyboard pane, optionally
// in voice mode. Used by the menu to auto-open the keyboard when launching apps
// flagged with `voice: true` in config.
func SendOpenKeyboard(voice bool) error {
	conn, err := net.Dial("unix", ControlSocketPath())
	if err != nil {
		return fmt.Errorf("dial control socket: %w", err)
	}
	defer conn.Close()
	msg := OpenKeyboardMessage{Type: MessageTypeOpenKeyboard, Voice: voice}
	return json.NewEncoder(conn).Encode(msg)
}
