package server

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/xzys/TR3X/v2/protocol"
)

const socketPath = "/tmp/tr3x.sock"

// Run starts the server and blocks until interrupted
func Run() error {
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

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("failed to accept connection", "err", err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		data := scanner.Bytes()
		if len(data) == 0 {
			continue
		}

		req, err := protocol.ParseRequest(data)
		if err != nil {
			slog.Error("failed to parse request", "err", err)
			sendError(conn, err.Error())
			continue
		}

		switch r := req.(type) {
		case protocol.PressKeyRequest:
			if err := PressKey(r.Key); err != nil {
				sendError(conn, err.Error())
				continue
			}
			sendSuccess(conn, &protocol.PressKeyResponse{Success: true})

		default:
			sendError(conn, "unknown request type")
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("scanner error", "err", err)
	}
}

func sendSuccess(conn net.Conn, response *protocol.PressKeyResponse) {
	sendJSON(conn, response)
}

func sendError(conn net.Conn, errorMsg string) {
	response := protocol.PressKeyResponse{
		Success: false,
		Error:   errorMsg,
	}
	sendJSON(conn, response)
}

func sendJSON(conn net.Conn, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("failed to marshal response", "err", err)
		return
	}
	data = append(data, '\n')
	if _, err := conn.Write(data); err != nil {
		slog.Error("failed to write response", "err", err)
	}
}
