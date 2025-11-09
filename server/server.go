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

	executor := NewChordExecutor()

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Error("failed to accept connection", "err", err)
			continue
		}

		go handleConnection(conn, executor)
	}
}

func handleConnection(conn net.Conn, executor *ChordExecutor) {
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
		case protocol.ExecuteChordRequest:
			if err := executor.ExecuteChord(r.Modifier, r.Trigger); err != nil {
				sendError(conn, err.Error())
				continue
			}
			sendSuccess(conn, &protocol.ExecuteChordResponse{Success: true})

		case protocol.GetStateRequest:
			modeShift, modeNumbers := executor.GetState()
			response := protocol.StateResponse{
				Success:     true,
				ModeShift:   modeShift,
				ModeNumbers: modeNumbers,
			}
			sendJSON(conn, response)

		default:
			sendError(conn, "unknown request type")
		}
	}

	if err := scanner.Err(); err != nil {
		slog.Error("scanner error", "err", err)
	}
}

func sendSuccess(conn net.Conn, response *protocol.ExecuteChordResponse) {
	sendJSON(conn, response)
}

func sendError(conn net.Conn, errorMsg string) {
	response := protocol.ExecuteChordResponse{
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

