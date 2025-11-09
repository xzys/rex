//go:build sdl
// +build sdl

package keyboard

// Mapping of the R36S controls

// The following table show the mapping of the joystick buttons and axes to the R36S controls:
// Joy-Button 	Description
// 0 	B
// 1 	A
// 2 	X
// 3 	Y
// 4 	L1
// 5 	R1
// 6 	L2
// 7 	R2
// 8 	D-Up
// 9 	D-Down
// 10 	D-Left
// 11 	D-Right
// 12 	Select
// 13 	Start
// 16 	Fn

// The axes are mapped as follows:
// Joy-Axis 	Description
// 0 	Left joystick - Left-Right motion
// 1 	Left joystick - Up-Down motion
// 2 	Right joystick - Left-Right motion
// 3 	Right joystick - Up-Down motion

import (
	"log"

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

	return pollJoystickEvents()
}

func pollJoystickEvents() tea.Cmd {
	return func() tea.Msg {
		event := sdl.PollEvent()
		if event != nil {
			return joystickMsg{event: event}
		}
		return nil
	}
}

func (m KeyboardView) Cleanup() {
	if m.model != nil {
		m.model.Cleanup()
	}
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
				case 0: // B button
					m.model.HandleKey(keyB)
				case 2: // X button
					m.model.HandleKey(keyX)
				case 3: // Y button
					m.model.HandleKey(keyY)
				case 4: // L1
					m.model.HandleKey(keyL1)
				case 5: // R1
					m.model.HandleKey(keyR1)
				case 6: // L2
					m.model.HandleKey(keyL2)
				case 7: // R2
					m.model.HandleKey(keyR2)
				}
			}
		}
		return m, pollJoystickEvents()
	}
	return m, nil
}
