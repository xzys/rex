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
	"github.com/xrfr/t-rex/v2/utils"
)

func TestStdinInputForwardsToKeyboard(t *testing.T) {
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

	// Open the FIFO read side before starting the server to avoid blocking.
	stdinFile, err := os.OpenFile(fifoPath, os.O_RDWR, 0600)
	if err != nil {
		t.Fatalf("open fifo for stdin: %v", err)
	}
	defer stdinFile.Close()

	// Start the server with stdin input wired to the FIFO.
	env := append(os.Environ(), "REX_INPUT_DEVICE=stdin")
	serverCmd, err := utils.StartRexProcess(ctx, rexPath, []string{"server"}, env, stdinFile, io.Discard, io.Discard)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer utils.TerminateProcess(serverCmd, 2*time.Second)

	// Wait for sockets so the keyboard can connect.
	if err := utils.WaitForSocket(protocol.InputSocketPath, 2*time.Second); err != nil {
		t.Fatalf("wait for input socket: %v", err)
	}
	if err := utils.WaitForSocket(protocol.OutputSocketPath, 2*time.Second); err != nil {
		t.Fatalf("wait for output socket: %v", err)
	}

	// Connect keyboard sockets so we can observe KeyInput messages.
	inputConn, err := net.Dial("unix", protocol.InputSocketPath)
	if err != nil {
		t.Fatalf("dial input socket: %v", err)
	}
	defer inputConn.Close()

	outputConn, err := net.Dial("unix", protocol.OutputSocketPath)
	if err != nil {
		t.Fatalf("dial output socket: %v", err)
	}
	defer outputConn.Close()

	if err := inputConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	// Write a stdin event to the FIFO to trigger a KeyInput message.
	writer, err := os.OpenFile(fifoPath, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatalf("open fifo writer: %v", err)
	}
	defer writer.Close()

	if _, err := fmt.Fprintln(writer, "down KeyA"); err != nil {
		t.Fatalf("write stdin event: %v", err)
	}

	msg, err := protocol.ParseMessage(json.NewDecoder(inputConn))
	if err != nil {
		t.Fatalf("read key input: %v", err)
	}

	keyMsg, ok := msg.(protocol.KeyInputMessage)
	if !ok {
		t.Fatalf("unexpected message type: %T", msg)
	}

	if keyMsg.KeyInput != protocol.KeyA || !keyMsg.IsKeyDown {
		t.Fatalf("unexpected key input: %+v", keyMsg)
	}
}
