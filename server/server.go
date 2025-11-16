package server

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/xzys/TR3X/v2/protocol"
)

var socketPath = protocol.SocketPath

type inputMode string

type Server struct {
	keyboardConn net.Conn
	mode         inputMode
}

const (
	inputModeGlobal   inputMode = "global"
	inputModeKeyboard inputMode = "keyboard"
)

func NewServer() *Server {
	return &Server{
		mode: inputModeGlobal,
	}
}

// Run starts the server and blocks until interrupted
func (s *Server) Run() error {
	slog.Info("starting server")
	// Remove socket file if it exists
	if _, err := os.Stat(socketPath); err == nil {
		if err := os.Remove(socketPath); err != nil {
			slog.Error("failed to remove existing socket", "err", err)
			return err
		}
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		slog.Error("failed to create socket", "err", err)
		return err
	}
	defer listener.Close()
	defer os.Remove(socketPath)

	slog.Info("server listening", "socket", socketPath)

	// Handle cleanup on interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		slog.Info("shutting down server")
		listener.Close()
		os.Remove(socketPath)
		os.Exit(0)
	}()

	if err := s.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Handle connections in a loop
	for {
		slog.Info("waiting for connection")
		if conn, err := listener.Accept(); err != nil {
			slog.Error("failed to accept connection", "err", err)
		} else {
			if err := s.handleConnection(conn); err != nil {
				slog.Error("failed to handle connection", "err", err)
			}
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) error {
	s.keyboardConn = conn
	s.mode = inputModeKeyboard
	// reset on exit
	defer func() {
		conn.Close()
		s.keyboardConn = nil
		s.mode = inputModeGlobal
	}()

	scanner := bufio.NewScanner(conn)
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
		case *protocol.PressKeyMessage:
			if err := pressKey(m.KeyPress); err != nil {
				return fmt.Errorf("failed to press key: %w", err)
			}
		}
	}
	// EOF will be nil, errors will be exposed
	return scanner.Err()
}
