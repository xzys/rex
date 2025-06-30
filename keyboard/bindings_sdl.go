//go:build sdl
// +build sdl

package keyboard

import (
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/veandco/go-sdl2/sdl"
)

type joystickMsg struct {
	event sdl.Event
}

// Global joystick reference for SDL build
var globalJoystick *sdl.Joystick

func (m KeyboardView) Init() tea.Cmd {
	// Initialize SDL
	if err := sdl.Init(sdl.INIT_JOYSTICK); err != nil {
		log.Fatal("Failed to initialize SDL: %v\n", err)
	}

	// Check for joysticks
	if sdl.NumJoysticks() < 1 {
		log.Fatal("Warning: No joysticks connected!")
	}

	// Open joystick and store in global variable
	globalJoystick = sdl.JoystickOpen(0)
	if globalJoystick == nil {
		log.Fatal("Warning: Unable to open game controller! SDL Error: %s\n", sdl.GetError())
	}

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

	return nil
}

func (m KeyboardView) Cleanup() {
	sdl.Quit()
	if globalJoystick != nil {
		globalJoystick.Close()
	}
}

func (m KeyboardView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
	case joystickMsg:
		switch e := msg.event.(type) {
		case *sdl.JoyButtonEvent:
			if e.Type == sdl.JOYBUTTONDOWN {
				switch e.Button {
				case 8: // D-Up
					m.model.HandleKey(keyDPadUp)
				case 9: // D-Down
					m.model.HandleKey(keyDPadDown)
				case 10: // D-Left
					m.model.HandleKey(keyDPadLeft)
				case 11: // D-Right
					m.model.HandleKey(keyDPadRight)
				case 1: // A button
					m.model.HandleKey(keyA)
				}
			}
		}
	}
	return m, nil
}
