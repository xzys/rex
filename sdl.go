//go:build sdl
// +build sdl

package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/veandco/go-sdl2/sdl"
)

type joystickMsg struct {
	event sdl.Event
}

// Global joystick reference for SDL build
var globalJoystick *sdl.Joystick

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
					m.moveSelection("up")
				case 9: // D-Down
					m.moveSelection("down")
				case 10: // D-Left
					m.moveSelection("left")
				case 11: // D-Right
					m.moveSelection("right")
				case 1: // A button
					m.handleKeySelection()
				}
			}
		}
	}
	return m, nil
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

	// Open joystick and store in global variable
	globalJoystick = sdl.JoystickOpen(0)
	if globalJoystick == nil {
		fmt.Printf("Warning: Unable to open game controller! SDL Error: %s\n", sdl.GetError())
		os.Exit(1)
	}
	defer globalJoystick.Close()

	p := tea.NewProgram(initialModel())

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
