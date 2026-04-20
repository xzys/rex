package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/xrfr/t-rex/v2/protocol"
)

type InputDevice interface {
	Initialize(func(InputEvent) error) error
}

var (
	inputDevice InputDevice
	binaryName  string
)

type Operator interface {
	HandleInput(key int, isDown bool) (bool, error)
}

type Server struct {
	encoder     *json.Encoder
	decoder     *json.Decoder
	inputDevice InputDevice
	operator    *GlobalOperator
	disabled    atomic.Bool
	voiceWS     *VoiceWebSocket
}

func NewServer() *Server {
	server := &Server{
		inputDevice: selectInputDevice(),
	}
	server.operator = NewGlobalOperator(server, map[string]Operator{
		"ncmpcpp": NewNcmpcppOperator(),
		"tmux":    NewSSHTmuxOperator(),
		"claude":  NewClaudeOperator(),
	})

	// Create voice WebSocket server for companion app.
	// tmuxInject sends keys to tmux pane 0 via the operator.
	server.voiceWS = NewVoiceWebSocket(func(keys string) error {
		return server.operator.HandleKeyPress(keys)
	})
	server.voiceWS.onClientChange = func(connected bool) {
		server.sendCompanionStatus(connected)
	}

	return server
}

func selectInputDevice() InputDevice {
	if strings.EqualFold(os.Getenv("REX_INPUT_DEVICE"), "stdin") {
		return NewStdinInputDevice(os.Stdin)
	}
	return inputDevice
}

// Run starts the server and blocks until interrupted
func (s *Server) Run() (runErr error) {
	slog.Info("starting server", "pid", os.Getpid())

	// Capture panics in Run's goroutine so we get a structured stack trace in
	// rex.log. Panics in spawned goroutines bypass this — they land in
	// /tmp/rex-stderr.log instead via debian-rex.sh's stderr redirect.
	defer func() {
		if r := recover(); r != nil {
			slog.Error("server panic", "panic", r, "stack", string(debug.Stack()))
			runErr = fmt.Errorf("panic: %v", r)
		}
	}()

	// get binary name
	if execPath, err := os.Executable(); err != nil {
		return fmt.Errorf("failed to get binary name: %w", err)
	} else {
		binaryName = execPath
	}

	// initialize input device, either keyboard or R36s
	if err := s.inputDevice.Initialize(s.operator.HandleInput); err != nil {
		return fmt.Errorf("failed to initialize input device: %w", err)
	}

	// remove socket files if they exist
	for _, path := range []string{protocol.InputSocketPath(), protocol.OutputSocketPath(), ControlSocketPath()} {
		if _, err := os.Stat(path); err == nil {
			os.Remove(path)
		}
	}

	// create Unix socket listeners
	inputListener, err := net.Listen("unix", protocol.InputSocketPath())
	if err != nil {
		slog.Error("failed to create input socket", "err", err)
		return err
	}
	defer inputListener.Close()
	defer os.Remove(protocol.InputSocketPath())

	outputListener, err := net.Listen("unix", protocol.OutputSocketPath())
	if err != nil {
		slog.Error("failed to create output socket", "err", err)
		return err
	}
	defer outputListener.Close()
	defer os.Remove(protocol.OutputSocketPath())

	controlListener, err := net.Listen("unix", ControlSocketPath())
	if err != nil {
		slog.Error("failed to create control socket", "err", err)
		return err
	}
	defer controlListener.Close()
	defer os.Remove(ControlSocketPath())

	slog.Info("server listening", "input", protocol.InputSocketPath(), "output", protocol.OutputSocketPath(), "control", ControlSocketPath())

	// Start voice WebSocket server for companion app
	if err := s.voiceWS.Start(VoiceWSPort); err != nil {
		slog.Error("failed to start voice websocket server", "err", err)
		// Non-fatal — server works without companion app
	}
	defer s.voiceWS.Stop()

	go s.handleControlConnections(controlListener)

	// handle cleanup on interrupt. Catch more signals than we plan to handle
	// so an unexpected signal kill (e.g. SIGHUP from parent exit) shows up in
	// the log instead of a silent death.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2)
	go func() {
		sig := <-sigChan
		slog.Info("shutting down server", "signal", sig.String())
		inputListener.Close()
		outputListener.Close()
		controlListener.Close()
		os.Remove(protocol.InputSocketPath())
		os.Remove(protocol.OutputSocketPath())
		os.Remove(ControlSocketPath())
		os.Exit(0)
	}()

	// handle connections in a loop
	for {
		slog.Info("waiting for connections")

		inputConn, err := inputListener.Accept()
		if err != nil {
			slog.Error("failed to accept input connection", "err", err)
			continue
		}

		outputConn, err := outputListener.Accept()
		if err != nil {
			inputConn.Close()
			slog.Error("failed to accept output connection", "err", err)
			continue
		}

		if err := s.handleConnections(inputConn, outputConn); err != nil {
			slog.Error("connection error", "err", err)
		}
	}
}

func (s *Server) handleConnections(inputConn, outputConn net.Conn) error {
	// read on decoder, write on encoder
	s.encoder = json.NewEncoder(inputConn)
	s.decoder = json.NewDecoder(outputConn)
	s.operator.SetKeyboardActive(true)

	defer func() {
		inputConn.Close()
		outputConn.Close()
		s.encoder = nil
		s.decoder = nil
		s.operator.SetKeyboardActive(false)
	}()

	slog.Info("keyboard connected")

	for {
		msg, err := protocol.ParseMessage(s.decoder)
		if err != nil {
			return fmt.Errorf("failed to parse message: %w", err)
		}

		switch m := msg.(type) {
		case protocol.PressKeyMessage:
			if err := s.operator.HandleKeyPress(m.KeyPress); err != nil {
				return fmt.Errorf("failed to press key: %w", err)
			}
		case protocol.PressStringMessage:
			if err := s.operator.HandlePressString(m.Value); err != nil {
				return fmt.Errorf("failed to press string: %w", err)
			}
		case protocol.VoiceCommandMessage:
			s.handleVoiceCommand(m.Action)
		}
	}
}

func (s *Server) handleVoiceCommand(action string) {
	switch action {
	case "start":
		if err := s.voiceWS.SendStart(); err != nil {
			slog.Error("voice command start failed", "err", err)
			return
		}
		slog.Info("voice: sent start to companion")
	case "stop":
		if err := s.voiceWS.SendStop(); err != nil {
			slog.Error("voice command stop failed", "err", err)
			return
		}
		slog.Info("voice: sent stop to companion, waiting for final")

		// Wait for final in a goroutine so we don't block the message loop
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			text, err := s.voiceWS.WaitForFinal(ctx)
			if err != nil {
				slog.Error("voice: wait for final failed", "err", err)
				// Send error back to keyboard
				s.sendVoiceText("", true)
				return
			}
			slog.Info("voice: received final transcription", "text", text)
			s.sendVoiceText(text, true)
		}()
	case "status":
		connected := s.voiceWS.HasClient()
		slog.Info("voice: status query", "connected", connected)
		s.sendCompanionStatus(connected)
	default:
		slog.Warn("unknown voice command action", "action", action)
	}
}

func (s *Server) sendCompanionStatus(connected bool) {
	if s.encoder == nil {
		return
	}
	msg := protocol.CompanionStatusMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeCompanionStatus},
		Connected:   connected,
	}
	if err := s.encoder.Encode(msg); err != nil {
		slog.Error("failed to send companion status to keyboard", "err", err)
	}
}

func (s *Server) sendVoiceText(text string, final bool) {
	if s.encoder == nil {
		return
	}
	msg := protocol.VoiceTextMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeVoiceText},
		Text:        text,
		Final:       final,
	}
	if err := s.encoder.Encode(msg); err != nil {
		slog.Error("failed to send voice text to keyboard", "err", err)
	}
}

func (s *Server) SendKeyInputMessage(event InputEvent) error {
	if s.encoder == nil {
		slog.Error("trying to send key input w/o encoder")
		return nil
	}

	msg := protocol.KeyInputMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput},
		KeyInput:    event.Key,
		IsKeyDown:   event.IsDown,
	}
	if err := s.encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to encode key input: %w", err)
	}
	slog.Debug("sent key input", "key", event.Key, "isDown", event.IsDown)
	return nil
}

func (s *Server) handleControlConnections(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("failed to accept control connection", "err", err)
			continue
		}
		go s.handleControlConnection(conn)
	}
}

func (s *Server) handleControlConnection(conn net.Conn) {
	defer conn.Close()
	decoder := json.NewDecoder(conn)
	for {
		msg, err := ParseControlMessage(decoder)
		if err != nil {
			return
		}
		switch m := msg.(type) {
		case DisableMessage:
			s.disabled.Store(m.Disable)
			slog.Info("disabled state changed", "disabled", m.Disable)
		case InjectInputMessage:
			if err := s.operator.HandleInput(InputEvent{Key: m.Key, IsDown: m.IsDown}); err != nil {
				slog.Error("inject input error", "err", err)
			}
		}
	}
}

func (s *Server) IsDisabled() bool {
	return s.disabled.Load()
}
