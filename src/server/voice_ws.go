package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

const VoiceWSPort = 9876

// WebSocket message types (R36S ↔ iOS)
type wsMessage struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// VoiceWebSocket manages the WebSocket server for the iOS companion app.
// It accepts one client at a time and relays start/stop commands from
// the voice model, receiving partial/final transcriptions back.
type VoiceWebSocket struct {
	mu       sync.Mutex
	conn     *websocket.Conn // current iOS client, nil if none
	ctx      context.Context
	cancel   context.CancelFunc
	server   *http.Server
	sentText string // smart replace state

	// finalCh delivers the final transcription text to the blocking Transcribe() call.
	finalCh chan string

	// tmuxInject is called to send keys to tmux. Injected for testability.
	tmuxInject func(keys string) error
}

// NewVoiceWebSocket creates a new voice WebSocket server.
// tmuxInject is the function used to inject keys into tmux (nil in tests).
func NewVoiceWebSocket(tmuxInject func(keys string) error) *VoiceWebSocket {
	ctx, cancel := context.WithCancel(context.Background())
	return &VoiceWebSocket{
		ctx:        ctx,
		cancel:     cancel,
		finalCh:    make(chan string, 1),
		tmuxInject: tmuxInject,
	}
}

// Start begins listening for WebSocket connections on the given port.
func (v *VoiceWebSocket) Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", v.handleWS)

	v.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
		BaseContext: func(net.Listener) context.Context {
			return v.ctx
		},
	}

	ln, err := net.Listen("tcp", v.server.Addr)
	if err != nil {
		return fmt.Errorf("voice ws listen: %w", err)
	}

	slog.Info("voice websocket server listening", "port", port)

	go func() {
		if err := v.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("voice ws server error", "err", err)
		}
	}()

	return nil
}

// Stop shuts down the WebSocket server.
func (v *VoiceWebSocket) Stop() {
	v.cancel()
	if v.server != nil {
		v.server.Close()
	}
}

// HasClient returns true if an iOS client is currently connected.
func (v *VoiceWebSocket) HasClient() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.conn != nil
}

// SendStart sends a start command to the connected iOS client.
func (v *VoiceWebSocket) SendStart() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.conn == nil {
		return fmt.Errorf("no companion client connected")
	}

	// Reset smart replace state for new transcription
	v.sentText = ""
	// Drain any stale final from previous session
	select {
	case <-v.finalCh:
	default:
	}

	return v.sendMsg(wsMessage{Type: "start"})
}

// SendStop sends a stop command to the connected iOS client.
func (v *VoiceWebSocket) SendStop() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.conn == nil {
		return fmt.Errorf("no companion client connected")
	}

	return v.sendMsg(wsMessage{Type: "stop"})
}

// WaitForFinal blocks until a final transcription is received from the iOS client.
// Returns the transcribed text or an error if the context is cancelled.
func (v *VoiceWebSocket) WaitForFinal(ctx context.Context) (string, error) {
	select {
	case text := <-v.finalCh:
		return text, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-v.ctx.Done():
		return "", v.ctx.Err()
	}
}

func (v *VoiceWebSocket) sendMsg(msg wsMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return v.conn.Write(v.ctx, websocket.MessageText, data)
}

func (v *VoiceWebSocket) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // accept connections from any origin
	})
	if err != nil {
		slog.Error("voice ws accept error", "err", err)
		return
	}

	v.mu.Lock()
	// Close any existing connection
	if v.conn != nil {
		v.conn.Close(websocket.StatusGoingAway, "new client connected")
	}
	v.conn = conn
	v.sentText = ""
	v.mu.Unlock()

	slog.Info("companion app connected", "remote", r.RemoteAddr)

	defer func() {
		v.mu.Lock()
		if v.conn == conn {
			v.conn = nil
			v.sentText = ""
		}
		v.mu.Unlock()
		conn.Close(websocket.StatusNormalClosure, "")
		slog.Info("companion app disconnected", "remote", r.RemoteAddr)
	}()

	// Read messages from iOS client
	for {
		_, data, err := conn.Read(v.ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				return
			}
			slog.Error("voice ws read error", "err", err)
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			slog.Error("voice ws unmarshal error", "err", err)
			continue
		}

		v.handleClientMessage(msg)
	}
}

func (v *VoiceWebSocket) handleClientMessage(msg wsMessage) {
	switch msg.Type {
	case "partial":
		v.applySmartReplace(msg.Text)
	case "final":
		v.applySmartReplace(msg.Text)
		// Deliver to WaitForFinal
		select {
		case v.finalCh <- msg.Text:
		default:
			slog.Warn("final channel full, dropping")
		}
	default:
		slog.Warn("unknown voice ws message type", "type", msg.Type)
	}
}

func (v *VoiceWebSocket) applySmartReplace(newText string) {
	v.mu.Lock()
	sent := v.sentText
	v.mu.Unlock()

	backspaces, toType := computeDelta(sent, newText)

	if v.tmuxInject != nil {
		// Send backspaces
		for i := 0; i < backspaces; i++ {
			if err := v.tmuxInject("BSpace"); err != nil {
				slog.Error("smart replace backspace failed", "err", err)
				return
			}
		}
		// Send new text
		if toType != "" {
			// Send character by character to handle special chars correctly
			for _, ch := range toType {
				if err := v.tmuxInject(string(ch)); err != nil {
					slog.Error("smart replace type failed", "err", err)
					return
				}
			}
		}
	}

	v.mu.Lock()
	v.sentText = newText
	v.mu.Unlock()

	if backspaces > 0 || toType != "" {
		slog.Debug("smart replace", "backspaces", backspaces, "typed", toType, "result", newText)
	}
}

// ResetState clears the smart replace state. Called on disconnect or error.
func (v *VoiceWebSocket) ResetState() {
	v.mu.Lock()
	v.sentText = ""
	v.mu.Unlock()

	// Drain any pending final
	select {
	case <-v.finalCh:
	default:
	}
}

// DiscardPartial clears any partial text that was injected into tmux
// by sending backspaces for the current sentText. Called on disconnect mid-transcription.
func (v *VoiceWebSocket) DiscardPartial() {
	v.mu.Lock()
	sent := v.sentText
	v.sentText = ""
	v.mu.Unlock()

	if v.tmuxInject != nil && len(sent) > 0 {
		runes := []rune(sent)
		for i := 0; i < len(runes); i++ {
			if err := v.tmuxInject("BSpace"); err != nil {
				slog.Error("discard partial backspace failed", "err", err)
				return
			}
		}
	}
}

// SentText returns the current smart replace state (for testing).
func (v *VoiceWebSocket) SentText() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.sentText
}

// Port returns the actual listening port. Useful when started on port 0 for tests.
func (v *VoiceWebSocket) Port() int {
	return VoiceWSPort
}
