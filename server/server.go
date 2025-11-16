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

type Server struct {
	keyboardConn *net.Conn
}

func NewServer() *Server {
	return &Server{}
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

	// Handle connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("failed to accept connection", "err", err)
			continue
		}

		if err := s.handleConnection(conn); err != nil {
			slog.Error("failed to handle connection", "err", err)
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) error {
	defer conn.Close()

	// Set the keyboard connection when established
	s.keyboardConn = &conn

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

	// Detect EOF vs errors explicitly
	if err := scanner.Err(); err != nil {
		// Clear connection on error
		s.keyboardConn = nil
		return fmt.Errorf("failed to scan message: %w", err)
	}

	// EOF detected (normal disconnect) - clear the connection
	s.keyboardConn = nil
	return nil
}
