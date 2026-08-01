package server

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/xrfr/t-rex/v2/stt"
)

// TestSTTSocketRoundTrip drives the daemon's transcribe socket end-to-end
// with the RemoteProvider client, using the stub model (!sherpa builds).
func TestSTTSocketRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("REX_STT_SOCKET", filepath.Join(dir, "stt.sock"))
	t.Setenv("REX_CONFIG_PATH", filepath.Join(dir, "config.yaml"))

	s := &Server{stt: newSTTEngine()}
	ln, err := net.Listen("unix", stt.SocketPath())
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go s.handleSTTConnections(ln)

	p := stt.NewRemoteProvider()
	defer p.Close()

	if err := p.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	text, err := p.Transcribe(make([]byte, 3200), 16000)
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text != "hello world" { // StubProvider's fixed response
		t.Fatalf("got %q, want %q", text, "hello world")
	}
}
