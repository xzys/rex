package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// startTestVoiceWS starts a VoiceWebSocket on a random port and returns it + the port.
func startTestVoiceWS(t *testing.T, tmuxInject func(string) error) (*VoiceWebSocket, int) {
	t.Helper()

	vws := NewVoiceWebSocket(tmuxInject)

	// Use a random port for tests
	mux := http.NewServeMux()
	mux.HandleFunc("/", vws.handleWS)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port

	vws.server = &http.Server{
		Handler: mux,
		BaseContext: func(net.Listener) context.Context {
			return vws.ctx
		},
	}

	go vws.server.Serve(ln)

	t.Cleanup(func() {
		vws.Stop()
	})

	return vws, port
}

// connectTestClient connects a WebSocket client to the test server.
func connectTestClient(t *testing.T, port int) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/", port), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	t.Cleanup(func() {
		conn.Close(websocket.StatusNormalClosure, "")
	})

	return conn
}

func readWSMessage(t *testing.T, conn *websocket.Conn) wsMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var msg wsMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return msg
}

func sendWSMessage(t *testing.T, conn *websocket.Conn, msg wsMessage) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func waitForClient(t *testing.T, vws *VoiceWebSocket) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if vws.HasClient() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for client to connect")
}

func TestVoiceWSClientConnect(t *testing.T) {
	vws, port := startTestVoiceWS(t, nil)

	if vws.HasClient() {
		t.Fatal("expected no client before connection")
	}

	connectTestClient(t, port)
	waitForClient(t, vws)

	if !vws.HasClient() {
		t.Fatal("expected client after connection")
	}
}

func TestVoiceWSSendStartStop(t *testing.T) {
	vws, port := startTestVoiceWS(t, nil)
	client := connectTestClient(t, port)
	waitForClient(t, vws)

	// Send start
	if err := vws.SendStart(); err != nil {
		t.Fatalf("SendStart: %v", err)
	}
	msg := readWSMessage(t, client)
	if msg.Type != "start" {
		t.Fatalf("expected start, got %q", msg.Type)
	}

	// Send stop
	if err := vws.SendStop(); err != nil {
		t.Fatalf("SendStop: %v", err)
	}
	msg = readWSMessage(t, client)
	if msg.Type != "stop" {
		t.Fatalf("expected stop, got %q", msg.Type)
	}
}

func TestVoiceWSReceiveFinal(t *testing.T) {
	vws, port := startTestVoiceWS(t, nil)
	client := connectTestClient(t, port)
	waitForClient(t, vws)

	// Client sends final
	sendWSMessage(t, client, wsMessage{Type: "final", Text: "hello world"})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	text, err := vws.WaitForFinal(ctx)
	if err != nil {
		t.Fatalf("WaitForFinal: %v", err)
	}
	if text != "hello world" {
		t.Fatalf("expected 'hello world', got %q", text)
	}
}

func TestVoiceWSSmartReplacePartials(t *testing.T) {
	var mu sync.Mutex
	var injected []string

	inject := func(keys string) error {
		mu.Lock()
		injected = append(injected, keys)
		mu.Unlock()
		return nil
	}

	vws, port := startTestVoiceWS(t, inject)
	client := connectTestClient(t, port)
	waitForClient(t, vws)

	// Send partials
	sendWSMessage(t, client, wsMessage{Type: "partial", Text: "hello"})
	time.Sleep(100 * time.Millisecond)

	sendWSMessage(t, client, wsMessage{Type: "partial", Text: "hello wor"})
	time.Sleep(100 * time.Millisecond)

	sendWSMessage(t, client, wsMessage{Type: "partial", Text: "hello world"})
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	result := strings.Join(injected, "")
	mu.Unlock()

	// Should have typed "hello wor" + "ld" = "hello world" (no backspaces)
	if result != "hello world" {
		t.Fatalf("expected injected 'hello world', got %q (keys: %v)", result, injected)
	}

	if vws.SentText() != "hello world" {
		t.Fatalf("expected sentText 'hello world', got %q", vws.SentText())
	}
}

func TestVoiceWSSmartReplaceCorrection(t *testing.T) {
	var mu sync.Mutex
	var injected []string

	inject := func(keys string) error {
		mu.Lock()
		injected = append(injected, keys)
		mu.Unlock()
		return nil
	}

	vws, port := startTestVoiceWS(t, inject)
	client := connectTestClient(t, port)
	waitForClient(t, vws)

	// "hello world" then correct to "hello word"
	sendWSMessage(t, client, wsMessage{Type: "partial", Text: "hello world"})
	time.Sleep(100 * time.Millisecond)

	sendWSMessage(t, client, wsMessage{Type: "partial", Text: "hello word"})
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	// Should see: h,e,l,l,o, ,w,o,r,l,d, BSpace, BSpace, d
	var bsCount int
	for _, k := range injected {
		if k == "BSpace" {
			bsCount++
		}
	}
	mu.Unlock()

	if bsCount != 2 {
		t.Fatalf("expected 2 backspaces, got %d (keys: %v)", bsCount, injected)
	}

	if vws.SentText() != "hello word" {
		t.Fatalf("expected sentText 'hello word', got %q", vws.SentText())
	}
}

func TestVoiceWSClientReconnect(t *testing.T) {
	vws, port := startTestVoiceWS(t, nil)

	// First connection
	client1 := connectTestClient(t, port)
	waitForClient(t, vws)

	// Second connection replaces first
	client2, _, err := websocket.Dial(context.Background(), fmt.Sprintf("ws://127.0.0.1:%d/", port), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client2.Close(websocket.StatusNormalClosure, "")

	time.Sleep(200 * time.Millisecond)

	// Server should work with new client
	if err := vws.SendStart(); err != nil {
		t.Fatalf("SendStart: %v", err)
	}

	// Read from client2 should work
	msg := readWSMessage(t, client2)
	if msg.Type != "start" {
		t.Fatalf("expected start on client2, got %q", msg.Type)
	}

	// client1 should have been closed — read should fail
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err = client1.Read(ctx)
	if err == nil {
		t.Fatal("expected error reading from replaced client1")
	}
	_ = client1.Close(websocket.StatusNormalClosure, "")
}

func TestVoiceWSNoClientError(t *testing.T) {
	vws, _ := startTestVoiceWS(t, nil)

	if err := vws.SendStart(); err == nil {
		t.Fatal("expected error when no client connected")
	}
	if err := vws.SendStop(); err == nil {
		t.Fatal("expected error when no client connected")
	}
}

func TestVoiceWSStartResetsState(t *testing.T) {
	var mu sync.Mutex
	var injected []string

	inject := func(keys string) error {
		mu.Lock()
		injected = append(injected, keys)
		mu.Unlock()
		return nil
	}

	vws, port := startTestVoiceWS(t, inject)
	client := connectTestClient(t, port)
	waitForClient(t, vws)

	// Simulate a partial transcription
	sendWSMessage(t, client, wsMessage{Type: "partial", Text: "hello"})
	time.Sleep(100 * time.Millisecond)

	if vws.SentText() != "hello" {
		t.Fatalf("expected sentText 'hello', got %q", vws.SentText())
	}

	// Read the start message from client side
	if err := vws.SendStart(); err != nil {
		t.Fatalf("SendStart: %v", err)
	}
	readWSMessage(t, client)

	// sentText should be reset
	if vws.SentText() != "" {
		t.Fatalf("expected empty sentText after SendStart, got %q", vws.SentText())
	}
}

func TestVoiceWSFullFlow(t *testing.T) {
	var mu sync.Mutex
	var injected []string

	inject := func(keys string) error {
		mu.Lock()
		injected = append(injected, keys)
		mu.Unlock()
		return nil
	}

	vws, port := startTestVoiceWS(t, inject)
	client := connectTestClient(t, port)
	waitForClient(t, vws)

	// 1. Server sends start
	if err := vws.SendStart(); err != nil {
		t.Fatalf("SendStart: %v", err)
	}
	msg := readWSMessage(t, client)
	if msg.Type != "start" {
		t.Fatalf("expected start, got %q", msg.Type)
	}

	// 2. Client streams partials
	sendWSMessage(t, client, wsMessage{Type: "partial", Text: "testing"})
	time.Sleep(100 * time.Millisecond)
	sendWSMessage(t, client, wsMessage{Type: "partial", Text: "testing one two"})
	time.Sleep(100 * time.Millisecond)

	// 3. Server sends stop
	if err := vws.SendStop(); err != nil {
		t.Fatalf("SendStop: %v", err)
	}
	msg = readWSMessage(t, client)
	if msg.Type != "stop" {
		t.Fatalf("expected stop, got %q", msg.Type)
	}

	// 4. Client sends final
	sendWSMessage(t, client, wsMessage{Type: "final", Text: "testing one two three"})

	// 5. WaitForFinal returns the text
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	text, err := vws.WaitForFinal(ctx)
	if err != nil {
		t.Fatalf("WaitForFinal: %v", err)
	}
	if text != "testing one two three" {
		t.Fatalf("expected 'testing one two three', got %q", text)
	}

	// 6. Verify smart replace injected the full text
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	result := strings.Join(injected, "")
	mu.Unlock()
	if result != "testing one two three" {
		t.Fatalf("expected injected 'testing one two three', got %q", result)
	}
}
