package keyboard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/xzys/TR3X/v2/protocol"
)

type Keyboard struct {
	keyboard [][]string
	model    KeyboardModel
	conn     net.Conn
	scanner  *bufio.Scanner
	width    int
}

type RuntimeError error

func getKeyboardModel(mode string) KeyboardModel {
	switch mode {
	case "chord":
		return NewKeyboardModelChorded(keyboardLayout)
	case "simple":
		return NewKeyboardModelSimple()
	default:
		slog.Error("invalid keyboard mode", "mode", mode)
		os.Exit(1)
		return nil
	}
}

func NewKeyboard(keyboardMode string) *Keyboard {
	model := getKeyboardModel(keyboardMode)
	return &Keyboard{
		keyboard: keyboardLayout,
		model:    model,
		conn:     nil,
		width:    0,
	}
}

func (k *Keyboard) Run() error {
	// close connection on exit
	defer func() {
		if k.conn != nil {
			k.conn.Close()
			k.conn = nil
		}
	}()

	// Run the UI
	p := tea.NewProgram(k)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// implement tea.Model interface Init, Update

func (k *Keyboard) Init() tea.Cmd {
	conn, err := net.Dial("unix", protocol.SocketPath)
	if err != nil {
		slog.Error("failed to connect to server", "err", err)
		os.Exit(1)
	}
	k.conn = conn
	k.scanner = bufio.NewScanner(conn)

	// 0. return tea.Cmd to read socket
	// 1. ReadSocket returns a message containing the parsed message or an error
	// 2. Update parses message and returns ReadSocket command again
	// 3. back to 1
	return k.ReadSocket
}

func (k *Keyboard) ReadSocket() tea.Msg {
	if !k.scanner.Scan() {
		if err := k.scanner.Err(); err != nil {
			return RuntimeError(fmt.Errorf("failed to scan message: %w", err))
		}
		return RuntimeError(fmt.Errorf("connection closed"))
	}

	data := k.scanner.Bytes()
	if len(data) == 0 {
		return k.ReadSocket()
	}

	msg, err := protocol.ParseMessage(data)
	if err != nil {
		return RuntimeError(fmt.Errorf("failed to parse message: %w", err))
	}
	return msg
}

func (k *Keyboard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return k, tea.Quit
		}
	case RuntimeError:
		slog.Error("runtime error", "err", msg)
		return k, tea.Quit
	case tea.WindowSizeMsg:
		k.width = msg.Width
	case protocol.KeyInputMessage:
		cmds = append(cmds, k.handleDeviceInput(msg))
	}

	// always read socket again after other cmds
	cmds = append(cmds, k.ReadSocket)
	return k, tea.Batch(cmds...)
}

// handle key input and optionally provide a command to send key press to server
func (k *Keyboard) handleDeviceInput(msg protocol.KeyInputMessage) tea.Cmd {
	// Process the key input through the model
	keyToPress := k.model.HandleKey(msg.KeyInput)
	if keyToPress == nil {
		// for ex. with modifier keys
		return nil
	}

	return func() tea.Msg {
		message := protocol.PressKeyMessage{
			BaseMessage: protocol.BaseMessage{Type: protocol.MessageTypeKeyPress},
			KeyPress:    *keyToPress,
		}
		messageData, err := json.Marshal(message)
		if err != nil {
			return RuntimeError(fmt.Errorf("failed to marshal message: %w", err))
		}
		messageData = append(messageData, '\n')
		if _, err := k.conn.Write(messageData); err != nil {
			return RuntimeError(fmt.Errorf("failed to write message: %w", err))
		}
		return nil
	}
}
