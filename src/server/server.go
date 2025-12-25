package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/xrfr/t-rex/v2/protocol"
)

type InputDevice interface {
	Initialize(func(int) error) error
}

var (
	inputDevice InputDevice
	binaryName  string
)

type Operator interface {
	HandleKey(key int) error
}

type Server struct {
	encoder     *json.Encoder
	decoder     *json.Decoder
	inputDevice InputDevice
	operator    *GlobalOperator
}

func NewServer() *Server {
	server := &Server{
		inputDevice: inputDevice,
	}
	server.operator = NewGlobalOperator(server, map[string]Operator{
		"ncmpcpp": NewNcmpcppOperator(),
		"menu":    NewMenuOperator(),
	})
	return server
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
	if err := s.inputDevice.Initialize(s.operator.HandleKey); err != nil {
		return fmt.Errorf("failed to initialize input device: %w", err)
	}

	// remove socket files if they exist
	for _, path := range []string{protocol.InputSocketPath, protocol.OutputSocketPath} {
		if _, err := os.Stat(path); err == nil {
			os.Remove(path)
		}
	}

	// create Unix socket listeners
	inputListener, err := net.Listen("unix", protocol.InputSocketPath)
	if err != nil {
		slog.Error("failed to create input socket", "err", err)
		return err
	}
	defer inputListener.Close()
	defer os.Remove(protocol.InputSocketPath)

	outputListener, err := net.Listen("unix", protocol.OutputSocketPath)
	if err != nil {
		slog.Error("failed to create output socket", "err", err)
		return err
	}
	defer outputListener.Close()
	defer os.Remove(protocol.OutputSocketPath)

	slog.Info("server listening", "input", protocol.InputSocketPath, "output", protocol.OutputSocketPath)

	// handle cleanup on interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		slog.Info("shutting down server")
		inputListener.Close()
		outputListener.Close()
		os.Remove(protocol.InputSocketPath)
		os.Remove(protocol.OutputSocketPath)
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
		}
	}
}

func (s *Server) SendKeyInputMessage(key int) error {
	if s.encoder == nil {
		slog.Error("trying to send key input w/o encoder")
		return nil
	}

	msg := protocol.KeyInputMessage{
		BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput},
		KeyInput:    key,
	}
	if err := s.encoder.Encode(msg); err != nil {
		return fmt.Errorf("failed to encode key input: %w", err)
	}
	slog.Debug("sent key input", "key", key)
	return nil
}
