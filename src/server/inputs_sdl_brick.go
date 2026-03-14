//go:build sdl && brick
// +build sdl,brick

package server

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/xrfr/t-rex/v2/protocol"
)

type SDLInputDevice struct{}

func init() {
	inputDevice = &SDLInputDevice{}
}

func (d *SDLInputDevice) Initialize(handler func(InputEvent) error) error {
	if err := sdl.Init(sdl.INIT_JOYSTICK); err != nil {
		return fmt.Errorf("failed to initialize SDL: %w", err)
	}

	if sdl.NumJoysticks() < 1 {
		return fmt.Errorf("no joysticks connected")
	}

	joystick := sdl.JoystickOpen(0)
	if joystick == nil {
		return fmt.Errorf("unable to open joystick: %s", sdl.GetError())
	}

	go runPollingLoop(joystick, handler)

	return nil
}

// TrimUI Brick raw SDL joystick button layout:
// b4=A, b5=B, b6=X, b7=Y
// b8=L1, b9=R1, b10=L2, b11=R2
// b12=Select, b13=Start, b14=Menu/Guide
// D-pad: hat 0 (SDL_JOYHATMOTION)

var brickButtonMap = map[uint8]int{
	4:  protocol.KeyA,
	5:  protocol.KeyB,
	6:  protocol.KeyX,
	7:  protocol.KeyY,
	8:  protocol.KeyL1,
	9:  protocol.KeyR1,
	10: protocol.KeyL2,
	11: protocol.KeyR2,
	12: protocol.KeySelect,
	13: protocol.KeyStart,
	14: protocol.KeyFn,
}

// lastHatKey tracks the currently-held d-pad direction so we can
// synthesize a key-up event when the hat returns to HAT_CENTERED.
var lastHatKey int

func runPollingLoop(joystick *sdl.Joystick, inputHandler func(InputEvent) error) {
	var err error
	defer func() {
		sdl.Quit()
		if joystick != nil {
			joystick.Close()
		}
		if err != nil {
			exec.Command("tmux", "kill-session", "-t", "rex").Run()
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
			err = handleBrickButton(e, inputHandler)
		case *sdl.JoyHatEvent:
			err = handleBrickHat(e, inputHandler)
		}

		if err != nil {
			slog.Error("failed to handle event", "err", err)
			return
		}
	}
}

func handleBrickButton(event *sdl.JoyButtonEvent, inputHandler func(InputEvent) error) error {
	key, ok := brickButtonMap[event.Button]
	if !ok {
		slog.Warn("unknown button", "button", event.Button)
		return nil
	}
	isDown := event.Type == sdl.JOYBUTTONDOWN
	return inputHandler(InputEvent{Key: key, IsDown: isDown})
}

func handleBrickHat(event *sdl.JoyHatEvent, inputHandler func(InputEvent) error) error {
	var key int
	switch event.Value {
	case sdl.HAT_UP:
		key = protocol.KeyDPadUp
	case sdl.HAT_DOWN:
		key = protocol.KeyDPadDown
	case sdl.HAT_LEFT:
		key = protocol.KeyDPadLeft
	case sdl.HAT_RIGHT:
		key = protocol.KeyDPadRight
	case sdl.HAT_CENTERED:
		if lastHatKey != 0 {
			if err := inputHandler(InputEvent{Key: lastHatKey, IsDown: false}); err != nil {
				return err
			}
			lastHatKey = 0
		}
		return nil
	default:
		// diagonal — ignore
		return nil
	}

	// send key-up for previous direction if it changed
	if lastHatKey != 0 && lastHatKey != key {
		if err := inputHandler(InputEvent{Key: lastHatKey, IsDown: false}); err != nil {
			return err
		}
	}
	lastHatKey = key
	return inputHandler(InputEvent{Key: key, IsDown: true})
}
