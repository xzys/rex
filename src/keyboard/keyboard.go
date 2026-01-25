package keyboard

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xrfr/t-rex/v2/protocol"
)

type Keyboard struct {
	keyboard [][]string
	model    KeyboardModel
	decoder  *json.Decoder
	encoder  *json.Encoder
	width    int
	// connections kept for cleanup
	inputConn  net.Conn
	outputConn net.Conn
}

type RuntimeError error

func getKeyboardModel(mode string) KeyboardModel {
	switch mode {
	case "chord":
		return NewKeyboardModelChorded(keyboardLayout)
	case "simple":
		return NewKeyboardModelSimple()
	default:
		panic("invalid keyboard mode")
	}
}

func NewKeyboard(keyboardMode string) *Keyboard {
	model := getKeyboardModel(keyboardMode)
	return &Keyboard{
		keyboard: keyboardLayout,
		model:    model,
		width:    0,
	}
}

func (k *Keyboard) Run() error {
	// close connections on exit
	defer func() {
		if k.inputConn != nil {
			k.inputConn.Close()
		}
		if k.outputConn != nil {
			k.outputConn.Close()
		}
	}()

	p := tea.NewProgram(k)
	_, err := p.Run()
	return err
}

// implement tea.Model interface Init, Update
var QuitCmd = tea.Sequence(tea.ExitAltScreen, tea.Quit)

func (k *Keyboard) Init() tea.Cmd {
	// Connect to input socket (receives KeyInput from server)
	inputConn, err := net.Dial("unix", protocol.InputSocketPath)
	if err != nil {
		slog.Error("failed to connect to input socket", "err", err)
		return QuitCmd
	}
	k.inputConn = inputConn
	k.decoder = json.NewDecoder(inputConn)

	// Connect to output socket (sends KeyPress to server)
	outputConn, err := net.Dial("unix", protocol.OutputSocketPath)
	if err != nil {
		slog.Error("failed to connect to output socket", "err", err)
		inputConn.Close()
		return QuitCmd
	}
	k.outputConn = outputConn
	k.encoder = json.NewEncoder(outputConn)

	slog.Info("connected to server", "input", protocol.InputSocketPath, "output", protocol.OutputSocketPath)

	// 0. return tea.Cmd to read socket
	// 1. ReadSocket returns a message containing the parsed message or an error
	// 2. Update parses message and returns ReadSocket command again
	// 3. back to 1
	return tea.EnterAltScreen
}

func (k *Keyboard) ReadSocket() tea.Msg {
	msg, err := protocol.ParseMessage(k.decoder)
	if err != nil {
		return RuntimeError(fmt.Errorf("failed to parse message: %w", err))
	}
	return msg
}

func (k *Keyboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// slog.Debug("main loop update", "type", fmt.Sprintf("%T", msg), "msg", msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return k, QuitCmd
		}
	case RuntimeError:
		slog.Error("runtime error", "err", msg)
		return k, QuitCmd
	case tea.WindowSizeMsg:
		k.width = msg.Width
		// NOTE somehow returning read socket here causes an issue
		return k, nil
	case protocol.KeyInputMessage:
		// Process the input and send response, then read next message
		return k, tea.Sequence(k.handleDeviceInput(msg), k.ReadSocket)
	}

	// For any other message type (including first message from Init), read from socket
	return k, k.ReadSocket
}

// handle key input and optionally provide a command to send key press to server
func (k *Keyboard) handleDeviceInput(msg protocol.KeyInputMessage) tea.Cmd {
	// Process the key input through the model (both keydown and keyup)
	keyToPress := k.model.HandleKey(msg.KeyInput, msg.IsKeyDown)
	if keyToPress == nil {
		// for ex. with modifier keys or keyup events
		return nil
	}

	return func() tea.Msg {
		message := protocol.PressKeyMessage{
			BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyPress},
			KeyPress:    *keyToPress,
		}
		if err := k.encoder.Encode(message); err != nil {
			return RuntimeError(fmt.Errorf("failed to encode message: %w", err))
		}
		return nil
	}
}
