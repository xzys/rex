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

// TrimUI Brick raw SDL joystick button layout.
// NOTE: indices depend on which libSDL2 rex is linked against.
// Debian's stock SDL2 uses upstream evdev enumeration. Knulli's bundled SDL2
// carries Batocera's sdl2_input_as_retroarch_udev.patch which shifts indices
// (phantom pre-BTN_JOYSTICK keys take low slots), pushing conventional buttons
// up by ~4. We run rex inside the Debian chroot against apt's libsdl2-2.0-0,
// so the layout below matches jstest on Debian SDL:
// b0=A(east), b1=B(south), b2=X(north), b3=Y(west)
// Physical labels use Nintendo layout (swapped vs Xbox):
//   b0 (hw "A") → KeyB (east), b1 (hw "B") → KeyA (south)
//   b2 (hw "X") → KeyY (north), b3 (hw "Y") → KeyX (west)
// b4=L1, b5=R1, b6=L2, b7=R2
// b8=Select, b9=Start, b10=Mode/Menu/Guide (KeyFn)
// b11 = left function button (KeyL3 — the Brick's left "FN" key, paired with b12)
// b12 = right function button (KeyR3 — the Brick's right "FN" key; types '/' globally)
// D-pad: hat 0 (SDL_JOYHATMOTION)
// TODO: joystick axis scroll (KeyScrollUp/KeyScrollDown via left-stick Y) is not yet
// implemented on Brick — the device has a left joystick but the axis handler used on
// R36S was never ported. Add handleBrickAxisEvent mirroring inputs_sdl.go when needed.

var brickButtonMap = map[uint8]int{
	0:  protocol.KeyB,
	1:  protocol.KeyA,
	2:  protocol.KeyY,
	3:  protocol.KeyX,
	4:  protocol.KeyL1,
	5:  protocol.KeyR1,
	6:  protocol.KeyL2,
	7:  protocol.KeyR2,
	8:  protocol.KeySelect,
	9:  protocol.KeyStart,
	10: protocol.KeyFn,
	11: protocol.KeyL3,
	12: protocol.KeyR3,
}

// lastHatKey tracks the currently-held d-pad direction so we can
// synthesize a key-up event when the hat returns to HAT_CENTERED.
// -1 means no key held (can't use 0 since protocol.KeyDPadUp == 0).
var lastHatKey = -1

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
			err = nil
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
		if lastHatKey != -1 {
			if err := inputHandler(InputEvent{Key: lastHatKey, IsDown: false}); err != nil {
				return err
			}
			lastHatKey = -1
		}
		return nil
	default:
		// diagonal — ignore
		return nil
	}

	// send key-up for previous direction (changed or repeated without HAT_CENTERED)
	if lastHatKey != -1 {
		if err := inputHandler(InputEvent{Key: lastHatKey, IsDown: false}); err != nil {
			return err
		}
	}
	lastHatKey = key
	return inputHandler(InputEvent{Key: key, IsDown: true})
}
