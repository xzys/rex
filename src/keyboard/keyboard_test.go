package keyboard_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
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

type inputEvent struct {
	key       int
	isKeyDown bool
}

type keyboardCase struct {
	name     string
	inputs   []inputEvent
	expected []string
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

var simpleLayout = [][]string{
	{"123", "q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "BCK"},
	{"SFT", "a", "s", "d", "f", "g", "h", "j", "k", "l", "SPC"},
	{"DEL", "z", "x", "c", "v", "b", "n", "m", "ENT"},
}

func TestKeyboardModes(t *testing.T) {
	modeSuites := []struct {
		name  string
		mode  string
		cases []keyboardCase
	}{
		{
			name: "chorded-toggle",
			mode: "chorded",
			cases: append(buildChordedCases(false),
				keyboardCase{
					name: "double_modifier_clears",
					inputs: []inputEvent{
						keyDown(protocol.KeyL1),
						keyDown(protocol.KeyL1),
						keyDown(protocol.KeyDPadUp),
					},
				},
			),
		},
		{
			name: "chorded-simultaneous",
			mode: "chorded-simultaneous",
			cases: append(buildChordedCases(true),
				keyboardCase{
					name: "modifier_keyup_clears",
					inputs: []inputEvent{
						keyDown(protocol.KeyL1),
						keyUp(protocol.KeyL1),
						keyDown(protocol.KeyDPadUp),
					},
				},
			),
		},
		{
			name: "simple",
			mode: "simple",
			cases: append(buildSimpleLetterCases(),
				keyboardCase{
					name: "select_enter",
					inputs: []inputEvent{
						keyDown(protocol.KeyDPadDown),
						keyDown(protocol.KeyDPadDown),
						keyDown(protocol.KeyDPadRight),
						keyDown(protocol.KeyDPadRight),
						keyDown(protocol.KeyDPadRight),
						keyDown(protocol.KeyDPadRight),
						keyDown(protocol.KeyDPadRight),
						keyDown(protocol.KeyDPadRight),
						keyDown(protocol.KeyDPadRight),
						keyDown(protocol.KeyDPadRight),
						keyDown(protocol.KeyA),
					},
					expected: []string{"\n"},
				},
			),
		},
	}

	for _, suite := range modeSuites {
		suite := suite
		t.Run(suite.name, func(t *testing.T) {
			for _, tc := range suite.cases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					runKeyboardCase(t, suite.mode, tc)
				})
			}
		})
	}
}

func buildChordedCases(includeKeyUp bool) []keyboardCase {
	cases := make([]keyboardCase, 0, len(chordTestCases))
	for _, tc := range chordTestCases {
		inputs := []inputEvent{
			keyDown(tc.modifier),
			keyDown(tc.trigger),
		}
		if includeKeyUp {
			inputs = append(inputs, keyUp(tc.modifier))
		}
		cases = append(cases, keyboardCase{
			name: fmt.Sprintf("mod%d_key%d_%s", tc.modifier, tc.trigger, tc.expected),
			inputs:   inputs,
			expected: []string{tc.expected},
		})
	}
	return cases
}

func buildSimpleLetterCases() []keyboardCase {
	var cases []keyboardCase
	for rowIdx, row := range simpleLayout {
		for colIdx, key := range row {
			if !isLowercaseLetter(key) {
				continue
			}
			inputs := buildSimpleInputsTo(rowIdx, colIdx)
			inputs = append(inputs, keyDown(protocol.KeyA))
			cases = append(cases, keyboardCase{
				name:     fmt.Sprintf("select_%s", key),
				inputs:   inputs,
				expected: []string{key},
			})
		}
	}
	return cases
}

func buildSimpleInputsTo(targetRow, targetCol int) []inputEvent {
	var inputs []inputEvent
	for i := 0; i < targetRow; i++ {
		inputs = append(inputs, keyDown(protocol.KeyDPadDown))
	}
	for i := 0; i < targetCol; i++ {
		inputs = append(inputs, keyDown(protocol.KeyDPadRight))
	}
	return inputs
}

func isLowercaseLetter(key string) bool {
	if len(key) != 1 {
		return false
	}
	r := rune(key[0])
	return r >= 'a' && r <= 'z'
}

func runKeyboardCase(t *testing.T, mode string, tc keyboardCase) {
	t.Helper()

	encoder, decoder, outputConn, cleanup := setupKeyboard(t, mode)
	defer cleanup()

	sendInputs(t, encoder, tc.inputs)
	assertOutputs(t, decoder, outputConn, tc.expected)
}

func setupKeyboard(t *testing.T, mode string) (*json.Encoder, *json.Decoder, net.Conn, func()) {
	t.Helper()

	socketDir, err := os.MkdirTemp("/tmp", "rex-test-")
	if err != nil {
		t.Fatalf("create socket temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(socketDir) })
	t.Setenv("REX_INPUT_SOCKET", filepath.Join(socketDir, "rex-input.sock"))
	t.Setenv("REX_OUTPUT_SOCKET", filepath.Join(socketDir, "rex-output.sock"))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	rexPath, cleanupBinary, err := utils.ResolveRexBinary(ctx)
	if err != nil {
		cancel()
		t.Fatalf("resolve rex binary: %v", err)
	}

	os.Remove(protocol.InputSocketPath())
	os.Remove(protocol.OutputSocketPath())

	inputListener, err := net.Listen("unix", protocol.InputSocketPath())
	if err != nil {
		cancel()
		cleanupBinary()
		t.Fatalf("create input socket listener: %v", err)
	}

	outputListener, err := net.Listen("unix", protocol.OutputSocketPath())
	if err != nil {
		cancel()
		cleanupBinary()
		inputListener.Close()
		t.Fatalf("create output socket listener: %v", err)
	}

	keyboardCmd, ptmx, err := utils.StartRexProcessWithPTY(ctx, rexPath, []string{"keyboard", "--mode", mode}, os.Environ())
	if err != nil {
		cancel()
		cleanupBinary()
		inputListener.Close()
		outputListener.Close()
		t.Fatalf("start keyboard: %v", err)
	}

	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := ptmx.Read(buf); err != nil {
				return
			}
		}
	}()

	inputListener.(*net.UnixListener).SetDeadline(time.Now().Add(5 * time.Second))
	inputConn, err := inputListener.Accept()
	if err != nil {
		cancel()
		cleanupBinary()
		inputListener.Close()
		outputListener.Close()
		ptmx.Close()
		utils.TerminateProcess(keyboardCmd, 2*time.Second)
		t.Fatalf("accept input connection: %v", err)
	}

	outputListener.(*net.UnixListener).SetDeadline(time.Now().Add(5 * time.Second))
	outputConn, err := outputListener.Accept()
	if err != nil {
		cancel()
		cleanupBinary()
		inputConn.Close()
		inputListener.Close()
		outputListener.Close()
		ptmx.Close()
		utils.TerminateProcess(keyboardCmd, 2*time.Second)
		t.Fatalf("accept output connection: %v", err)
	}

	encoder := json.NewEncoder(inputConn)
	decoder := json.NewDecoder(outputConn)

	cleanup := func() {
		outputConn.Close()
		inputConn.Close()
		inputListener.Close()
		outputListener.Close()
		ptmx.Close()
		utils.TerminateProcess(keyboardCmd, 2*time.Second)
		os.Remove(protocol.InputSocketPath())
		os.Remove(protocol.OutputSocketPath())
		cleanupBinary()
		cancel()
	}

	return encoder, decoder, outputConn, cleanup
}

func sendInputs(t *testing.T, encoder *json.Encoder, inputs []inputEvent) {
	t.Helper()

	for _, input := range inputs {
		msg := protocol.KeyInputMessage{
			BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput},
			KeyInput:    input.key,
			IsKeyDown:   input.isKeyDown,
		}
		if err := encoder.Encode(msg); err != nil {
			t.Fatalf("encode input: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func assertOutputs(t *testing.T, decoder *json.Decoder, outputConn net.Conn, expected []string) {
	t.Helper()

	for _, want := range expected {
		got, ok, err := readPressKey(decoder, outputConn, 2*time.Second)
		if err != nil {
			t.Fatalf("read key press: %v", err)
		}
		if !ok {
			t.Fatalf("expected key press %q, got none", want)
		}
		if got != want {
			t.Fatalf("expected key press %q, got %q", want, got)
		}
	}

	got, ok, err := readPressKey(decoder, outputConn, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("read extra key press: %v", err)
	}
	if ok {
		t.Fatalf("expected no extra key press, got %q", got)
	}
}

func readPressKey(decoder *json.Decoder, outputConn net.Conn, timeout time.Duration) (string, bool, error) {
	if err := outputConn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return "", false, err
	}

	response, err := protocol.ParseMessage(decoder)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return "", false, nil
		}
		return "", false, err
	}

	pressMsg, ok := response.(protocol.PressKeyMessage)
	if !ok {
		return "", false, fmt.Errorf("unexpected message type: %T", response)
	}

	return pressMsg.KeyPress, true, nil
}

func keyDown(key int) inputEvent {
	return inputEvent{key: key, isKeyDown: true}
}

func keyUp(key int) inputEvent {
	return inputEvent{key: key, isKeyDown: false}
}
