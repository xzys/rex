package server

import (
	"encoding/json"
	"fmt"
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

const MessageTypeDisable = "disable"

type DisableMessage struct {
	Type    string `json:"type"`
	Disable bool   `json:"disable"`
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
	default:
		return nil, fmt.Errorf("unknown message type: %s", msgType)
	}
}
