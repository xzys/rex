package keyboard_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/xrfr/t-rex/v2/protocol"
	"github.com/xrfr/t-rex/v2/utils"
)

type chordTestCase struct {
	modifier int
	trigger  int
	expected string
}

var chordTestCases = []chordTestCase{
	// R2 modifier - DPad triggers
	{protocol.KeyR2, protocol.KeyDPadLeft, "a"},
	{protocol.KeyR2, protocol.KeyDPadUp, "s"},
	{protocol.KeyR2, protocol.KeyDPadDown, "d"},
	{protocol.KeyR2, protocol.KeyDPadRight, "f"},
	// R2 modifier - Face button triggers
	{protocol.KeyR2, protocol.KeyY, "h"},
	{protocol.KeyR2, protocol.KeyX, "j"},
	{protocol.KeyR2, protocol.KeyB, "k"},
	{protocol.KeyR2, protocol.KeyA, "l"},

	// L1 modifier - DPad triggers
	{protocol.KeyL1, protocol.KeyDPadLeft, "q"},
	{protocol.KeyL1, protocol.KeyDPadUp, "w"},
	{protocol.KeyL1, protocol.KeyDPadDown, "e"},
	{protocol.KeyL1, protocol.KeyDPadRight, "r"},
	// L1 modifier - Face button triggers
	{protocol.KeyL1, protocol.KeyY, "u"},
	{protocol.KeyL1, protocol.KeyX, "i"},
	{protocol.KeyL1, protocol.KeyB, "o"},
	{protocol.KeyL1, protocol.KeyA, "p"},

	// R1 modifier - DPad triggers
	{protocol.KeyR1, protocol.KeyDPadLeft, "z"},
	{protocol.KeyR1, protocol.KeyDPadUp, "x"},
	{protocol.KeyR1, protocol.KeyDPadDown, "c"},
	{protocol.KeyR1, protocol.KeyDPadRight, "v"},
	// R1 modifier - Face button triggers
	{protocol.KeyR1, protocol.KeyY, "g"},
	{protocol.KeyR1, protocol.KeyX, "b"},
	{protocol.KeyR1, protocol.KeyB, "n"},
	{protocol.KeyR1, protocol.KeyA, "m"},

	// L2 modifier - DPad triggers (only up/down produce letters)
	{protocol.KeyL2, protocol.KeyDPadUp, "t"},
	{protocol.KeyL2, protocol.KeyDPadDown, "y"},
}

func TestChordedKeyboardAllLetters(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rexPath, cleanup, err := utils.ResolveRexBinary(ctx)
	if err != nil {
		t.Fatalf("resolve rex binary: %v", err)
	}
	defer cleanup()

	// Remove existing sockets if present
	os.Remove(protocol.InputSocketPath)
	os.Remove(protocol.OutputSocketPath)

	// Test acts as the server: create socket listeners
	inputListener, err := net.Listen("unix", protocol.InputSocketPath)
	if err != nil {
		t.Fatalf("create input socket listener: %v", err)
	}
	defer inputListener.Close()
	defer os.Remove(protocol.InputSocketPath)

	outputListener, err := net.Listen("unix", protocol.OutputSocketPath)
	if err != nil {
		t.Fatalf("create output socket listener: %v", err)
	}
	defer outputListener.Close()
	defer os.Remove(protocol.OutputSocketPath)

	// Start keyboard process in PTY
	// Capture PTY output to see any errors
	keyboardCmd, ptmx, err := utils.StartRexProcessWithPTY(ctx, rexPath, []string{"keyboard", "--mode", "chord"}, os.Environ())
	if err != nil {
		t.Fatalf("start keyboard: %v", err)
	}
	defer ptmx.Close()
	defer utils.TerminateProcess(keyboardCmd, 2*time.Second)
	
	// Read PTY output to prevent buffer from filling up
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := ptmx.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Accept keyboard's connections (keyboard connects to input first, then output)
	inputListener.(*net.UnixListener).SetDeadline(time.Now().Add(5 * time.Second))
	inputConn, err := inputListener.Accept()
	if err != nil {
		t.Fatalf("accept input connection: %v", err)
	}
	defer inputConn.Close()

	outputListener.(*net.UnixListener).SetDeadline(time.Now().Add(5 * time.Second))
	outputConn, err := outputListener.Accept()
	if err != nil {
		t.Fatalf("accept output connection: %v", err)
	}
	defer outputConn.Close()

	encoder := json.NewEncoder(inputConn)
	decoder := json.NewDecoder(outputConn)

	for _, tc := range chordTestCases {
		name := fmt.Sprintf("mod%d_key%d_%s", tc.modifier, tc.trigger, tc.expected)
		t.Run(name, func(t *testing.T) {
			if err := outputConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatalf("set read deadline: %v", err)
			}

			// Send modifier key down if needed
			if tc.modifier >= 0 {
				msg := protocol.KeyInputMessage{
					BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput},
					KeyInput:    tc.modifier,
					IsKeyDown:   true,
				}
				if err := encoder.Encode(msg); err != nil {
					t.Fatalf("encode modifier: %v", err)
				}
				// Delay to prevent JSON decoder sync issues in Bubble Tea
				time.Sleep(50 * time.Millisecond)
			}

			// Send trigger key down
			msg := protocol.KeyInputMessage{
				BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput},
				KeyInput:    tc.trigger,
				IsKeyDown:   true,
			}
			if err := encoder.Encode(msg); err != nil {
				t.Fatalf("encode trigger: %v", err)
			}
			
			// Small delay to let keyboard process and respond
			time.Sleep(10 * time.Millisecond)

			// Read PressKeyMessage from keyboard
			response, err := protocol.ParseMessage(decoder)
			if err != nil {
				t.Fatalf("read key press: %v", err)
			}

			pressMsg, ok := response.(protocol.PressKeyMessage)
			if !ok {
				t.Fatalf("unexpected message type: %T", response)
			}

			if pressMsg.KeyPress != tc.expected {
				t.Errorf("expected key press %q, got %q", tc.expected, pressMsg.KeyPress)
			}
		})
	}
}
