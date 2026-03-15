package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"

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
	HandleKey(key int) (bool, error)
}

type Server struct {
	encoder     *json.Encoder
	decoder     *json.Decoder
	inputDevice InputDevice
	operator    *GlobalOperator
	disabled    atomic.Bool
}

func NewServer() *Server {
	server := &Server{
		inputDevice: selectInputDevice(),
	}
	server.operator = NewGlobalOperator(server, map[string]Operator{
		"ncmpcpp": NewNcmpcppOperator(),
		"menu":    NewMenuOperator(),
	})
	return server
}

func selectInputDevice() InputDevice {
	if strings.EqualFold(os.Getenv("REX_INPUT_DEVICE"), "stdin") {
		return NewStdinInputDevice(os.Stdin)
	}
	return inputDevice
}

// Run starts the server and blocks until interrupted
func (s *Server) Run() error {
	slog.Info("starting server")

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

	go s.handleControlConnections(controlListener)

	// handle cleanup on interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		slog.Info("shutting down server")
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
		}
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
