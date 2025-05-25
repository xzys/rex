package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/veandco/go-sdl2/sdl"
)

const (
	_colorPrimary   = lipgloss.Color("#d7dadc")
	_colorSecondary = lipgloss.Color("#626262")
	_colorYellow    = lipgloss.Color("#b59f3b")
	_colorGreen     = lipgloss.Color("#538d4e")
)

type keyState int

const (
	_keyStateUnselected keyState = iota
	_keyStateAbsent
	_keyStatePresent
	_keyStateCorrect
)

func (s keyState) color() lipgloss.Color {
	switch s {
	case _keyStateUnselected:
		return _colorPrimary
	case _keyStateAbsent:
		return _colorSecondary
	case _keyStatePresent:
		return _colorYellow
	case _keyStateCorrect:
		return _colorGreen
	default:
		return _colorPrimary
	}
}

type model struct {
	keyStates   map[string]keyState
	selectedRow int
	selectedCol int
	keyboard    [][]string
	joystick    *sdl.Joystick
}

type joystickMsg struct {
	event sdl.Event
}

func initialModel(joystick *sdl.Joystick) model {
	keyboard := [][]string{
		{"Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P"},
		{"A", "S", "D", "F", "G", "H", "J", "K", "L"},
		{"ENTER", "Z", "X", "C", "V", "B", "N", "M", "DELETE"},
	}
	return model{
		keyStates:   make(map[string]keyState),
		selectedRow: 0,
		selectedCol: 0,
		keyboard:    keyboard,
		joystick:    joystick,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	case joystickMsg:
		switch e := msg.event.(type) {
		case *sdl.JoyButtonEvent:
			if e.Type == sdl.JOYBUTTONDOWN {
				switch e.Button {
				case 8: // D-Up
					if m.selectedRow > 0 {
						m.selectedRow--
						if m.selectedCol >= len(m.keyboard[m.selectedRow]) {
							m.selectedCol = len(m.keyboard[m.selectedRow]) - 1
						}
					}
				case 9: // D-Down
					if m.selectedRow < len(m.keyboard)-1 {
						m.selectedRow++
						if m.selectedCol >= len(m.keyboard[m.selectedRow]) {
							m.selectedCol = len(m.keyboard[m.selectedRow]) - 1
						}
					}
				case 10: // D-Left
					if m.selectedCol > 0 {
						m.selectedCol--
					}
				case 11: // D-Right
					if m.selectedCol < len(m.keyboard[m.selectedRow])-1 {
						m.selectedCol++
					}
				case 1: // A button
					key := m.keyboard[m.selectedRow][m.selectedCol]
					if len(key) == 1 {
						currentState := m.keyStates[key]
						m.keyStates[key] = (currentState + 1) % 4
						fmt.Printf("Selected key: %s, new state: %d\n", key, m.keyStates[key])
					} else {
						fmt.Printf("Selected special key: %s\n", key)
					}
				}
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	var rows []string
	for i, row := range m.keyboard {
		var keys []string
		for j, key := range row {
			status := _keyStateUnselected
			if len(key) == 1 {
				if state, exists := m.keyStates[key]; exists {
					status = state
				}
			}
			style := lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.NormalBorder()).
				BorderForeground(status.color()).
				Foreground(status.color())

			// Highlight selected key
			if i == m.selectedRow && j == m.selectedCol {
				style = style.BorderStyle(lipgloss.DoubleBorder())
			}

			keys = append(keys, style.Render(key))
		}
		rowStr := lipgloss.JoinHorizontal(lipgloss.Bottom, keys...)
		if i == 0 {
			rowStr = lipgloss.NewStyle().Padding(0, 2).Render(rowStr)
		} else if i == 1 {
			rowStr = lipgloss.NewStyle().Padding(0, 4).Render(rowStr)
		}
		rows = append(rows, rowStr)
	}

	keys := lipgloss.JoinVertical(lipgloss.Left, rows...)
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(_keyStateUnselected.color()).
		Padding(0, 1).
		Render(keys)
}

func main() {
	// Initialize SDL
	if err := sdl.Init(sdl.INIT_JOYSTICK); err != nil {
		fmt.Printf("Failed to initialize SDL: %v\n", err)
		os.Exit(1)
	}
	defer sdl.Quit()

	// Check for joysticks
	if sdl.NumJoysticks() < 1 {
		fmt.Println("Warning: No joysticks connected!")
		os.Exit(1)
	}

	// Open joystick
	joystick := sdl.JoystickOpen(0)
	if joystick == nil {
		fmt.Printf("Warning: Unable to open game controller! SDL Error: %s\n", sdl.GetError())
		os.Exit(1)
	}
	defer joystick.Close()

	p := tea.NewProgram(initialModel(joystick))

	// Start event polling in a goroutine
	go func() {
		for {
			event := sdl.PollEvent()
			if event != nil {
				// Send event to the Bubble Tea program
				p.Send(joystickMsg{event: event})
			}
			time.Sleep(16 * time.Millisecond) // ~60 FPS
		}
	}()

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
