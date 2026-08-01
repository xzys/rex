package stt

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

// The daemon serves transcription over a Unix socket so other rex processes
// (dictator) can use its resident model instead of loading their own copy.

const defaultSocketPath = "/tmp/rex-stt.sock"

// SocketPath returns the transcribe-service socket path (REX_STT_SOCKET
// overrides, for tests).
func SocketPath() string {
	if p := os.Getenv("REX_STT_SOCKET"); p != "" {
		return p
	}
	return defaultSocketPath
}

// Request is one JSON line sent to the daemon's stt socket.
type Request struct {
	Cmd        string `json:"cmd"` // "status" or "transcribe"
	SampleRate int    `json:"sample_rate,omitempty"`
	PCM        []byte `json:"pcm,omitempty"` // raw S16_LE mono, base64-encoded on the wire
}

// Response is the daemon's reply to a Request.
type Response struct {
	Ready bool   `json:"ready,omitempty"` // status: model loaded
	Mode  string `json:"mode,omitempty"`  // status: active stt_mode
	Text  string `json:"text,omitempty"`  // transcribe: result
	Error string `json:"error,omitempty"`
}

// RemoteProvider implements Provider by sending PCM to the daemon's stt
// socket. Capture and segmentation stay with the caller; only recognition
// runs in the daemon.
type RemoteProvider struct {
	mu   sync.Mutex
	conn net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
}

func NewRemoteProvider() *RemoteProvider {
	return &RemoteProvider{}
}

// Initialize polls the daemon until it reports the model loaded. The daemon
// warm-loads at startup, so this normally returns on the first poll.
func (p *RemoteProvider) Initialize() error {
	deadline := time.Now().Add(2 * time.Minute)
	for {
		resp, err := p.roundTrip(Request{Cmd: "status"}, 10*time.Second)
		if err == nil && resp.Ready {
			return nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("stt daemon unreachable at %s: %w", SocketPath(), err)
			}
			return fmt.Errorf("stt daemon: model not ready")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func (p *RemoteProvider) Transcribe(audio []byte, sampleRate int) (string, error) {
	resp, err := p.roundTrip(Request{Cmd: "transcribe", SampleRate: sampleRate, PCM: audio}, 60*time.Second)
	if err != nil {
		return "", err
	}
	if resp.Error != "" {
		return "", fmt.Errorf("stt daemon: %s", resp.Error)
	}
	return resp.Text, nil
}

// Close drops the socket connection. The daemon keeps its model loaded.
func (p *RemoteProvider) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
}

func (p *RemoteProvider) roundTrip(req Request, timeout time.Duration) (Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn == nil {
		conn, err := net.DialTimeout("unix", SocketPath(), 5*time.Second)
		if err != nil {
			return Response{}, err
		}
		p.conn = conn
		p.enc = json.NewEncoder(conn)
		p.dec = json.NewDecoder(conn)
	}

	p.conn.SetDeadline(time.Now().Add(timeout))
	var resp Response
	err := p.enc.Encode(req)
	if err == nil {
		err = p.dec.Decode(&resp)
	}
	if err != nil {
		// Dead connection (daemon restarted?) — drop it so the next call redials.
		p.conn.Close()
		p.conn = nil
		return Response{}, fmt.Errorf("stt socket: %w", err)
	}
	return resp, nil
}
