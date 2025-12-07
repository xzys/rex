//go:build sdl
// +build sdl

package server

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/xrfr/t-rex/v2/protocol"
)

type SDLInputDevice struct{}

func init() {
	inputDevice = &SDLInputDevice{}
}

func (d *SDLInputDevice) Initialize(handler func(int) error) error {
	// Initialize SDL
	if err := sdl.Init(sdl.INIT_JOYSTICK); err != nil {
		return fmt.Errorf("failed to initialize SDL: %w", err)
	}

	// Check for joysticks
	if sdl.NumJoysticks() < 1 {
		return fmt.Errorf("no joysticks connected")
	}

	// Open joystick
	joystick := sdl.JoystickOpen(0)
	if joystick == nil {
		return fmt.Errorf("unable to open game controller: %s", sdl.GetError())
	}

	go runPollingLoop(joystick, handler)

	return nil
}

func runPollingLoop(joystick *sdl.Joystick, inputHandler func(key int) error) {
	var err error
	defer func() {
		sdl.Quit()
		if joystick != nil {
			joystick.Close()
		}
		if err != nil {
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}()

	for {
		event := sdl.WaitEvent()
		if event == nil {
			continue
		}

		switch e := event.(type) {
		case *sdl.JoyButtonEvent:
			if err = handleJoystickEvent(e, inputHandler); err != nil {
				slog.Error("failed to handle joystick event", "err", err)
				return
			}
		}
	}
}

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

func handleJoystickEvent(event *sdl.JoyButtonEvent, inputHandler func(key int) error) error {
	switch event.Type {
	case sdl.JOYBUTTONDOWN:
		var key int
		switch event.Button {
		case 8: // D-Up
			key = protocol.KeyDPadUp
		case 9: // D-Down
			key = protocol.KeyDPadDown
		case 10: // D-Left
			key = protocol.KeyDPadLeft
		case 11: // D-Right
			key = protocol.KeyDPadRight
		case 1: // A button
			key = protocol.KeyA
		case 0: // B button
			key = protocol.KeyB
		case 2: // X button
			key = protocol.KeyX
		case 3: // Y button
			key = protocol.KeyY
		case 4: // L1
			key = protocol.KeyL1
		case 5: // R1
			key = protocol.KeyR1
		case 6: // L2
			key = protocol.KeyL2
		case 7: // R2
			key = protocol.KeyR2
		case 12: // Select
			key = protocol.KeySelect
		case 13: // Start
			key = protocol.KeyStart
		case 16: // Fn
			key = protocol.KeyFn
		default:
			slog.Warn("unknown button", "button", event.Button)
			return nil
		}
		if err := inputHandler(key); err != nil {
			return fmt.Errorf("failed to handle button: %w", err)
		}
	default:
		// other events like button up?
		slog.Warn("unknown event type", "event", event)
	}
	return nil
}
