package server

import (
	"encoding/json"
	"log/slog"
	"net"

	cfg "github.com/xrfr/t-rex/v2/config"
	"github.com/xrfr/t-rex/v2/stt"
)

// handleSTTConnections serves the daemon's transcribe socket: other rex
// processes (dictator) send PCM segments and get text back from the resident
// model, so they never load a model of their own.
func (s *Server) handleSTTConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return // listener closed on shutdown
		}
		go s.serveSTTClient(conn)
	}
}

func (s *Server) serveSTTClient(conn net.Conn) {
	defer conn.Close()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)
	for {
		var req stt.Request
		if err := dec.Decode(&req); err != nil {
			return // client hung up
		}
		if err := enc.Encode(s.handleSTTRequest(req)); err != nil {
			return
		}
	}
}

func (s *Server) handleSTTRequest(req stt.Request) stt.Response {
	// Companion mode routes through the iPhone mic — not usable for socket
	// clients, so fall back to the on-device default.
	mode := currentSTTMode()
	if mode == cfg.STTModeCompanion {
		mode = cfg.STTModeParakeet
	}
	s.stt.Ensure(mode)

	switch req.Cmd {
	case "status":
		return stt.Response{Ready: s.stt.Ready(), Mode: mode}
	case "transcribe":
		text, err := s.stt.Transcribe(req.PCM, req.SampleRate)
		if err != nil {
			slog.Error("stt socket: transcribe failed", "err", err)
			return stt.Response{Error: err.Error()}
		}
		return stt.Response{Text: text}
	default:
		return stt.Response{Error: "unknown cmd: " + req.Cmd}
	}
}
