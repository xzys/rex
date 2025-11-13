package protocol

import "encoding/json"

// Request types
const (
	RequestTypePressKey = "press_key"
)

// PressKeyRequest represents a request to press a key
type PressKeyRequest struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

// PressKeyResponse represents the response from pressing a key
type PressKeyResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ParseRequest parses a JSON request message
func ParseRequest(data []byte) (interface{}, error) {
	var base map[string]interface{}
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, err
	}

	reqType, ok := base["type"].(string)
	if !ok {
		return nil, json.Unmarshal(data, &PressKeyRequest{})
	}

	switch reqType {
	case RequestTypePressKey:
		var req PressKeyRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
		return req, nil
	default:
		return nil, json.Unmarshal(data, &PressKeyRequest{})
	}
}
