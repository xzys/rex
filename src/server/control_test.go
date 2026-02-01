package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/xrfr/t-rex/v2/protocol"
	"github.com/xrfr/t-rex/v2/server"
	"github.com/xrfr/t-rex/v2/utils"
)

func TestControlSocketDisable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rexPath, cleanup, err := utils.ResolveRexBinary(ctx)
	if err != nil {
		t.Fatalf("resolve rex binary: %v", err)
	}
	defer cleanup()

	fifoPath, fifoCleanup, err := utils.CreateTempFIFO()
	if err != nil {
		t.Fatalf("create fifo: %v", err)
	}
	defer fifoCleanup()

	stdinFile, err := os.OpenFile(fifoPath, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("open fifo for stdin: %v", err)
	}
	defer stdinFile.Close()

	env := append(os.Environ(), "REX_INPUT_DEVICE=stdin")
	serverCmd, err := utils.StartRexProcess(ctx, rexPath, []string{"server"}, env, stdinFile, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer utils.TerminateProcess(serverCmd, 2*time.Second)

	// Wait for all sockets
	if err := utils.WaitForSocket(protocol.InputSocketPath(), 2*time.Second); err != nil {
		t.Fatalf("wait for input socket: %v", err)
	}
	if err := utils.WaitForSocket(protocol.OutputSocketPath(), 2*time.Second); err != nil {
		t.Fatalf("wait for output socket: %v", err)
	}
	if err := utils.WaitForSocket(server.ControlSocketPath(), 2*time.Second); err != nil {
		t.Fatalf("wait for control socket: %v", err)
	}

	// Connect as keyboard client
	inputConn, err := net.Dial("unix", protocol.InputSocketPath())
	if err != nil {
		t.Fatalf("dial input socket: %v", err)
	}
	defer inputConn.Close()

	outputConn, err := net.Dial("unix", protocol.OutputSocketPath())
	if err != nil {
		t.Fatalf("dial output socket: %v", err)
	}
	defer outputConn.Close()

	// Connect to control socket
	controlConn, err := net.Dial("unix", server.ControlSocketPath())
	if err != nil {
		t.Fatalf("dial control socket: %v", err)
	}
	defer controlConn.Close()

	controlEncoder := json.NewEncoder(controlConn)

	writer, err := os.OpenFile(fifoPath, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open fifo writer: %v", err)
	}
	defer writer.Close()

	// Start a persistent reader goroutine that sends messages to a channel
	msgChan := make(chan protocol.KeyInputMessage, 10)
	go func() {
		decoder := json.NewDecoder(inputConn)
		for {
			msg, err := protocol.ParseMessage(decoder)
			if err != nil {
				return
			}
			if keyMsg, ok := msg.(protocol.KeyInputMessage); ok {
				msgChan <- keyMsg
			}
		}
	}()

	// Helper to read with timeout from channel
	readKeyInput := func(timeout time.Duration) (*protocol.KeyInputMessage, error) {
		select {
		case msg := <-msgChan:
			return &msg, nil
		case <-time.After(timeout):
			return nil, fmt.Errorf("timeout")
		}
	}

	// Phase 1: Verify input works normally
	t.Log("Phase 1: Verify normal operation")

	if _, err := fmt.Fprintln(writer, "down KeyA"); err != nil {
		t.Fatalf("write stdin event: %v", err)
	}

	keyMsg, err := readKeyInput(2 * time.Second)
	if err != nil {
		t.Fatalf("read key input (phase 1): %v", err)
	}
	if keyMsg.KeyInput != protocol.KeyA || !keyMsg.IsKeyDown {
		t.Fatalf("unexpected key input: %+v", keyMsg)
	}
	t.Log("Phase 1: Received KeyA as expected")

	// Phase 2: Disable input and verify it's ignored
	t.Log("Phase 2: Disable input")
	disableMsg := server.DisableMessage{
		Type:    server.MessageTypeDisable,
		Disable: true,
	}
	if err := controlEncoder.Encode(disableMsg); err != nil {
		t.Fatalf("send disable message: %v", err)
	}

	// Delay to ensure server processes the disable message
	time.Sleep(100 * time.Millisecond)

	// Send another key event - should be ignored
	if _, err := fmt.Fprintln(writer, "down KeyB"); err != nil {
		t.Fatalf("write stdin event (phase 2): %v", err)
	}

	// Expect timeout since input is disabled
	_, err = readKeyInput(300 * time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout but received message while disabled")
	}
	if err.Error() != "timeout" {
		t.Fatalf("expected timeout error, got: %v", err)
	}
	t.Log("Phase 2: Input correctly ignored while disabled")

	// Phase 3: Re-enable and verify input works again
	t.Log("Phase 3: Re-enable input")
	enableMsg := server.DisableMessage{
		Type:    server.MessageTypeDisable,
		Disable: false,
	}
	if err := controlEncoder.Encode(enableMsg); err != nil {
		t.Fatalf("send enable message: %v", err)
	}

	// Delay to ensure server processes the enable message
	time.Sleep(100 * time.Millisecond)

	if _, err := fmt.Fprintln(writer, "down KeyX"); err != nil {
		t.Fatalf("write stdin event (phase 3): %v", err)
	}

	keyMsg, err = readKeyInput(2 * time.Second)
	if err != nil {
		t.Fatalf("read key input (phase 3): %v", err)
	}
	if keyMsg.KeyInput != protocol.KeyX || !keyMsg.IsKeyDown {
		t.Fatalf("unexpected key input: %+v", keyMsg)
	}
	t.Log("Phase 3: Received KeyX as expected after re-enable")
}
