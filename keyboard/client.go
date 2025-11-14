package keyboard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"

	"github.com/xzys/TR3X/v2/protocol"
)

const socketPath = "/tmp/tr3x.sock"

// Client handles communication with the server
type Client struct {
	conn  net.Conn
	model KeyboardModel
}

// NewClient creates a new client connection to the server
func NewClient(model KeyboardModel) (*Client, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn, model: model}, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// HandleMessages listens for KeyInputMessage from server and responds with PressKeyMessage
func (c *Client) HandleMessages() error {
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) == 0 {
			continue
		}

		msg, err := protocol.ParseMessage(data)
		if err != nil {
			return fmt.Errorf("failed to parse message: %w", err)
		}

		switch m := msg.(type) {
		case *protocol.KeyInputMessage:
			// Process the key input through the model
			keyToPress, err := c.model.HandleKey(m.KeyInput)
			if err != nil {
				return fmt.Errorf("failed to handle key: %w", err)
			}

			// Respond with PressKeyMessage if needed
			if keyToPress != nil {
				message := protocol.PressKeyMessage{
					BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyPress},
					KeyPress:    *keyToPress,
				}
				messageData, err := json.Marshal(message)
				if err != nil {
					return fmt.Errorf("failed to marshal message: %w", err)
				}
				messageData = append(messageData, '\n')
				if _, err := c.conn.Write(messageData); err != nil {
					return fmt.Errorf("failed to write message: %w", err)
				}
			}
		default:
			return fmt.Errorf("unknown message type")
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan message: %w", err)
	}
	return nil
}
