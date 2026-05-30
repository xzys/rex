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

type inputEvent struct {
	key       int
	isKeyDown bool
}

type keyboardCase struct {
	name     string
	inputs   []inputEvent
	expected []string
}

var simpleLayout = [][]string{
	// Mirror the on-screen simple layout to drive navigation tests.
	{"q", "w", "e", "r", "u", "i", "o", "p"},
	{"a", "s", "d", "f", "h", "j", "k", "l"},
	{"z", "x", "c", "v", "b", "n", "m", "MIC"},
	{"123", "SFT", "g", "t", "y", "ENT", "DEL", "SPC"},
}

// SFT and 123 are layer toggles consumed locally by the keyboard — they emit
// nothing to the server — so they're absent here. Layer behavior is covered by
// the model unit tests (TestSimpleNumbersToggle, TestSimpleShiftSticky).
var simpleSpecialKeyOutputs = map[string]string{
	"SPC": string(protocol.SpecialKeySpace),
	"ENT": string(protocol.SpecialKeyEnter),
	"DEL": string(protocol.SpecialKeyDelete),
}

// TestSimpleKeyboard verifies the simple (cursor-driven) layout: letters and
// special keys (space/enter/delete/shift/numbers).
func TestSimpleKeyboard(t *testing.T) {
	cases := append(append(
		buildSimpleLetterCases(),
		buildSimpleSpecialCases()...),
		keyboardCase{
			name: "select_enter",
			inputs: []inputEvent{
				keyDown(protocol.KeyDPadDown),
				keyDown(protocol.KeyDPadDown),
				keyDown(protocol.KeyDPadDown),
				keyDown(protocol.KeyDPadRight),
				keyDown(protocol.KeyDPadRight),
				keyDown(protocol.KeyDPadRight),
				keyDown(protocol.KeyDPadRight),
				keyDown(protocol.KeyDPadRight),
				keyDown(protocol.KeyA),
			},
			expected: []string{"\n"},
		},
	)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runKeyboardCase(t, "simple", tc)
		})
	}
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

func buildSimpleSpecialCases() []keyboardCase {
	var cases []keyboardCase
	for rowIdx, row := range simpleLayout {
		for colIdx, key := range row {
			expected, ok := simpleSpecialKeyOutputs[key]
			if !ok {
				continue
			}
			inputs := buildSimpleInputsTo(rowIdx, colIdx)
			inputs = append(inputs, keyDown(protocol.KeyA))
			cases = append(cases, keyboardCase{
				name:     fmt.Sprintf("select_%s", key),
				inputs:   inputs,
				expected: []string{expected},
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
	if _, ok := os.LookupEnv("REX_CONFIG_PATH"); !ok {
		t.Setenv("REX_CONFIG_PATH", filepath.Join(socketDir, "config.yaml"))
	}

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

	switch msg := response.(type) {
	case protocol.PressKeyMessage:
		return msg.KeyPress, true, nil
	case protocol.PressStringMessage:
		return msg.Value, true, nil
	default:
		return "", false, fmt.Errorf("unexpected message type: %T", response)
	}
}

// Front-chorded chord helper: press mod down, trigger down, trigger up, mod up
func frontChordInputs(mod, trigger int) []inputEvent {
	return []inputEvent{
		keyDown(mod),
		keyDown(trigger),
		keyUp(trigger),
		keyUp(mod),
	}
}

func TestFrontChordedKeyboard(t *testing.T) {
	// Enable autocomplete via config so the keyboard process loads the dictionary
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	os.WriteFile(configPath, []byte("keyboard_autocomplete: true\n"), 0644)
	t.Setenv("REX_CONFIG_PATH", configPath)

	cases := []keyboardCase{
		{
			name:     "type_t_produces_t",
			inputs:   frontChordInputs(protocol.KeyDPadRight, protocol.KeyA),
			expected: []string{"t"},
		},
		{
			name: "type_th_produces_t_h",
			inputs: append(
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyA),   // t
				frontChordInputs(protocol.KeyY, protocol.KeyDPadLeft)..., // h
			),
			expected: []string{"t", "h"},
		},
		{
			name: "autocomplete_th_L1_selects_the",
			inputs: append(append(
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyA),   // t
				frontChordInputs(protocol.KeyY, protocol.KeyDPadLeft)..., // h
			), keyDown(protocol.KeyL1)), // L1 selects first suggestion
			expected: []string{"t", "h", "e "},
		},
		{
			name: "autocomplete_th_L2_selects_second",
			inputs: append(append(
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyA),
				frontChordInputs(protocol.KeyY, protocol.KeyDPadLeft)...,
			), keyDown(protocol.KeyL2)),
			// "that" is second suggestion (rank 6 in words.txt) → remaining = "at "
			expected: []string{"t", "h", "at "},
		},
		{
			name: "no_autocomplete_after_space",
			inputs: append(append(append(
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyA),    // t
				frontChordInputs(protocol.KeyA, protocol.KeyDPadRight)..., // space
			),
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyA)..., // t again
			), keyDown(protocol.KeyL1)), // L1 selects first suggestion for "t"
			// After space, new word "t" → suggestions restart → "the" → remaining "he "
			expected: []string{"t", " ", "t", "he "},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runKeyboardCase(t, "front-chorded", tc)
		})
	}
}

func keyDown(key int) inputEvent {
	return inputEvent{key: key, isKeyDown: true}
}

func keyUp(key int) inputEvent {
	return inputEvent{key: key, isKeyDown: false}
}
