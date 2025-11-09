package keyboard

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"os"

	"github.com/xzys/TR3X/v2/protocol"
)

const socketPath = "/tmp/tr3x.sock"

// Client handles communication with the server
type Client struct {
	conn net.Conn
}

// NewClient creates a new client connection to the server
func NewClient() (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ExecuteChord sends a chord execution request to the server
func (c *Client) ExecuteChord(modifier, trigger int) error {
	req := protocol.ExecuteChordRequest{
		Type:     protocol.RequestTypeExecuteChord,
		Modifier: modifier,
		Trigger:  trigger,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		return err
	}

	// Read response
	scanner := bufio.NewScanner(c.conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return err
		}
		return os.ErrClosed
	}

	var response protocol.ExecuteChordResponse
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		return err
	}

	if !response.Success {
		errorMsg := response.Error
		if errorMsg == "" {
			errorMsg = "unknown error"
		}
		slog.Error("chord execution failed", "error", errorMsg)
		return os.ErrInvalid
	}

	return nil
}

// GetState retrieves the current state from the server
func (c *Client) GetState() (bool, bool, error) {
	req := protocol.GetStateRequest{
		Type: protocol.RequestTypeGetState,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return false, false, err
	}
	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		return false, false, err
	}

	// Read response
	scanner := bufio.NewScanner(c.conn)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, false, err
		}
		return false, false, os.ErrClosed
	}

	var response protocol.StateResponse
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		return false, false, err
	}

	if !response.Success {
		errorMsg := response.Error
		if errorMsg == "" {
			errorMsg = "unknown error"
		}
		return false, false, os.ErrInvalid
	}

	return response.ModeShift, response.ModeNumbers, nil
}

