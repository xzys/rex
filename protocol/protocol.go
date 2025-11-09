package protocol

import "encoding/json"

// Request types
const (
	RequestTypeExecuteChord = "execute_chord"
	RequestTypeGetState     = "get_state"
)

// ExecuteChordRequest represents a request to execute a chord
type ExecuteChordRequest struct {
	Type     string `json:"type"`
	Modifier int    `json:"modifier"`
	Trigger  int    `json:"trigger"`
}

// GetStateRequest represents a request to get current state
type GetStateRequest struct {
	Type string `json:"type"`
}

// StateResponse represents the current state
type StateResponse struct {
	Success     bool   `json:"success"`
	ModeShift   bool   `json:"mode_shift,omitempty"`
	ModeNumbers bool   `json:"mode_numbers,omitempty"`
	Error       string `json:"error,omitempty"`
}

// ExecuteChordResponse represents the response from executing a chord
type ExecuteChordResponse struct {
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
		return nil, json.Unmarshal(data, &ExecuteChordRequest{})
	}

	switch reqType {
	case RequestTypeExecuteChord:
		var req ExecuteChordRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
		return req, nil
	case RequestTypeGetState:
		var req GetStateRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
		return req, nil
	default:
		return nil, json.Unmarshal(data, &ExecuteChordRequest{})
	}
}

