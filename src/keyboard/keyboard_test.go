package keyboard_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
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
	// Base chord map (letters), used across modes.
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
	// Mirror the on-screen simple layout to drive navigation tests.
	{"q", "w", "e", "r", "u", "i", "o", "p"},
	{"a", "s", "d", "f", "h", "j", "k", "l"},
	{"z", "x", "c", "v", "b", "n", "m", "MIC"},
	{"123", "SFT", "g", "t", "y", "ENT", "DEL", "SPC"},
}

var simpleSpecialKeyOutputs = map[string]string{
	"SPC": string(protocol.SpecialKeySpace),
	"ENT": string(protocol.SpecialKeyEnter),
	"DEL": string(protocol.SpecialKeyDelete),
	"SFT": string(protocol.SpecialKeyShift),
	"123": string(protocol.SpecialKeyNumbers),
}

var chordedNumberCases = []chordTestCase{
	{protocol.KeyR2, protocol.KeyDPadLeft, "1"},
	{protocol.KeyR2, protocol.KeyDPadUp, "2"},
	{protocol.KeyR2, protocol.KeyDPadDown, "3"},
	{protocol.KeyR2, protocol.KeyDPadRight, "4"},
	{protocol.KeyR2, protocol.KeyY, "5"},
	{protocol.KeyR2, protocol.KeyX, "6"},
	{protocol.KeyR2, protocol.KeyB, "7"},
	{protocol.KeyR2, protocol.KeyA, "8"},
}

var chordedSpecialCases = []chordTestCase{
	{protocol.KeyL2, protocol.KeyDPadRight, string(protocol.SpecialKeySpace)},
	{protocol.KeyL2, protocol.KeyX, string(protocol.SpecialKeyEnter)},
	{protocol.KeyL2, protocol.KeyY, string(protocol.SpecialKeyDelete)},
}

// TestKeyboardModes verifies chorded (toggle and simultaneous) and simple layouts,
// covering letters, digits 1-8, and special keys (space/enter/delete/close/shift/numbers).
func TestKeyboardModes(t *testing.T) {
	modeSuites := []struct {
		name  string
		mode  string
		cases []keyboardCase
	}{
		{
			name: "chorded-toggle",
			mode: "chorded",
			cases: append(append(append(append(
				buildChordedCases(false),
				buildChordedNumberCases(false)...),
				buildChordedSpecialCases(false)...),
				buildChordedShiftCases(false)...),
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
			cases: append(append(append(append(
				buildChordedCases(true),
				buildChordedNumberCases(true)...),
				buildChordedSpecialCases(true)...),
				buildChordedShiftCases(true)...),
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
			cases: append(append(
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
			name:     fmt.Sprintf("mod%d_key%d_%s", tc.modifier, tc.trigger, tc.expected),
			inputs:   inputs,
			expected: []string{tc.expected},
		})
	}
	return cases
}

func buildChordedNumberCases(includeKeyUp bool) []keyboardCase {
	return buildChordedModeCases("numbers", chordedNumberCases, includeKeyUp, func() []inputEvent {
		inputs := []inputEvent{
			keyDown(protocol.KeyL2),
			keyDown(protocol.KeyB),
		}
		if includeKeyUp {
			inputs = append(inputs, keyUp(protocol.KeyL2))
		}
		return inputs
	})
}

func buildChordedSpecialCases(includeKeyUp bool) []keyboardCase {
	return buildChordedModeCases("special", chordedSpecialCases, includeKeyUp, nil)
}

func buildChordedShiftCases(includeKeyUp bool) []keyboardCase {
	var inputs []inputEvent
	inputs = append(inputs, keyDown(protocol.KeyL2), keyDown(protocol.KeyDPadLeft))
	if includeKeyUp {
		inputs = append(inputs, keyUp(protocol.KeyL2))
	}

	expected := make([]string, 0, len(chordTestCases))
	for _, tc := range chordTestCases {
		inputs = append(inputs, keyDown(tc.modifier), keyDown(tc.trigger))
		if includeKeyUp {
			inputs = append(inputs, keyUp(tc.modifier))
		}
		expected = append(expected, strings.ToUpper(tc.expected))
	}

	inputs = append(inputs, keyDown(protocol.KeyL2), keyDown(protocol.KeyDPadLeft))
	if includeKeyUp {
		inputs = append(inputs, keyUp(protocol.KeyL2))
	}

	return []keyboardCase{
		{
			name:     "shift_mode_letters",
			inputs:   inputs,
			expected: expected,
		},
	}
}

func buildChordedModeCases(prefix string, cases []chordTestCase, includeKeyUp bool, preface func() []inputEvent) []keyboardCase {
	var inputs []inputEvent
	if preface != nil {
		inputs = append(inputs, preface()...)
	}

	// Flatten multiple chords into one input stream to validate order and state.
	expected := make([]string, 0, len(cases))
	for _, tc := range cases {
		inputs = append(inputs, keyDown(tc.modifier), keyDown(tc.trigger))
		if includeKeyUp {
			inputs = append(inputs, keyUp(tc.modifier))
		}
		expected = append(expected, tc.expected)
	}

	return []keyboardCase{
		{
			name:     fmt.Sprintf("%s_mode_chords", prefix),
			inputs:   inputs,
			expected: expected,
		},
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
			inputs:   frontChordInputs(protocol.KeyDPadRight, protocol.KeyY),
			expected: []string{"t"},
		},
		{
			name: "type_th_produces_t_h",
			inputs: append(
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyY), // t
				frontChordInputs(protocol.KeyA, protocol.KeyDPadUp)..., // h
			),
			expected: []string{"t", "h"},
		},
		{
			name: "autocomplete_th_L1_selects_the",
			inputs: append(append(
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyY), // t
				frontChordInputs(protocol.KeyA, protocol.KeyDPadUp)..., // h
			), keyDown(protocol.KeyL1)), // L1 selects first suggestion
			expected: []string{"t", "h", "e "},
		},
		{
			name: "autocomplete_th_L2_selects_second",
			inputs: append(append(
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyY),
				frontChordInputs(protocol.KeyA, protocol.KeyDPadUp)...,
			), keyDown(protocol.KeyL2)),
			// "that" is second suggestion (rank 6 in words.txt) → remaining = "at "
			expected: []string{"t", "h", "at "},
		},
		{
			name: "no_autocomplete_after_space",
			inputs: append(append(append(
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyY),   // t
				frontChordInputs(protocol.KeyX, protocol.KeyDPadLeft)..., // space
			),
				frontChordInputs(protocol.KeyDPadRight, protocol.KeyY)..., // t again
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

// TestRapidInputNoPanic sends many chords with no delay between messages to
// reproduce the concurrent json.Decoder access bug (json decoder out of sync).
// The keyboard process must survive and produce correct output for every chord.
func TestRapidInputNoPanic(t *testing.T) {
	encoder, decoder, outputConn, cleanup := setupKeyboard(t, "chorded")
	defer cleanup()

	// 30 rapid R2+A chords sent with no inter-message delay
	const n = 30
	go func() {
		for i := 0; i < n; i++ {
			for _, msg := range []protocol.KeyInputMessage{
				{BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput}, KeyInput: protocol.KeyR2, IsKeyDown: true},
				{BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput}, KeyInput: protocol.KeyA, IsKeyDown: true},
				{BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput}, KeyInput: protocol.KeyA, IsKeyDown: false},
				{BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyInput}, KeyInput: protocol.KeyR2, IsKeyDown: false},
			} {
				if err := encoder.Encode(msg); err != nil {
					return
				}
			}
		}
	}()

	for i := 0; i < n; i++ {
		got, ok, err := readPressKey(decoder, outputConn, 5*time.Second)
		if err != nil {
			t.Fatalf("chord %d: read key press: %v", i, err)
		}
		if !ok {
			t.Fatalf("chord %d: expected key press 'l', got none (keyboard may have panicked)", i)
		}
		if got != "l" {
			t.Fatalf("chord %d: expected 'l', got %q", i, got)
		}
	}
}
