//go:build sdl && !brick
// +build sdl,!brick

package server

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	"github.com/xrfr/t-rex/v2/protocol"
)

type SDLInputDevice struct{}

func init() {
	inputDevice = &SDLInputDevice{}
}

func (d *SDLInputDevice) Initialize(handler func(InputEvent) error) error {
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

func runPollingLoop(joystick *sdl.Joystick, inputHandler func(InputEvent) error) {
	var err error
	leftStickYDir := 0 // -1 = up, +1 = down, 0 = centered/deadzone
	repeater := newKeyRepeater(inputHandler)
	defer func() {
		repeater.StopAll()
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
			if err = handleJoystickEvent(e, inputHandler, repeater); err != nil {
				slog.Error("failed to handle joystick event", "err", err)
				return
			}
		case *sdl.JoyAxisEvent:
			if err = handleJoystickAxisEvent(e, inputHandler, &leftStickYDir, repeater); err != nil {
				slog.Error("failed to handle joystick axis event", "err", err)
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
// 14 	L3 (left stick click)
// 15 	R3 (right stick click) — types '/'
// 16 	Fn

// The axes are mapped as follows:
// Joy-Axis 	Description
// 0 	Left joystick - Left-Right motion
// 1 	Left joystick - Up-Down motion (mapped to tmux scroll up/down)
// 2 	Right joystick - Left-Right motion
// 3 	Right joystick - Up-Down motion

const (
	leftStickYAxis         = 1
	leftStickAxisThreshold = 16000
	repeatDelay            = 260 * time.Millisecond
	repeatInterval         = 90 * time.Millisecond
)

type keyRepeater struct {
	handler func(InputEvent) error

	mu   sync.Mutex
	key  int
	stop chan struct{}
}

func newKeyRepeater(handler func(InputEvent) error) *keyRepeater {
	return &keyRepeater{handler: handler, key: -1}
}

func (r *keyRepeater) Start(key int) {
	r.mu.Lock()
	if r.key == key && r.stop != nil {
		r.mu.Unlock()
		return
	}
	r.stopLocked()
	stop := make(chan struct{})
	r.key = key
	r.stop = stop
	r.mu.Unlock()

	go func() {
		timer := time.NewTimer(repeatDelay)
		defer timer.Stop()
		for {
			select {
			case <-stop:
				return
			case <-timer.C:
				if err := r.handler(InputEvent{Key: key, IsDown: true}); err != nil {
					slog.Warn("repeat key dispatch failed", "key", key, "err", err)
				}
				timer.Reset(repeatInterval)
			}
		}
	}()
}

func (r *keyRepeater) Stop(key int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.key != key {
		return
	}
	r.stopLocked()
}

func (r *keyRepeater) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopLocked()
}

func (r *keyRepeater) stopLocked() {
	if r.stop != nil {
		close(r.stop)
		r.stop = nil
	}
	r.key = -1
}

func handleJoystickEvent(event *sdl.JoyButtonEvent, inputHandler func(InputEvent) error, repeater *keyRepeater) error {
	isDown := event.Type == sdl.JOYBUTTONDOWN
	if event.Type != sdl.JOYBUTTONDOWN && event.Type != sdl.JOYBUTTONUP {
		slog.Warn("unknown event type", "event", event)
		return nil
	}

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
	case 15: // R3 (right stick click)
		key = protocol.KeyR3
	case 16: // Fn
		key = protocol.KeyFn
	default:
		slog.Warn("unknown button", "button", event.Button)
		return nil
	}
	if key == protocol.KeyDPadLeft || key == protocol.KeyDPadRight {
		if isDown {
			if err := inputHandler(InputEvent{Key: key, IsDown: true}); err != nil {
				return fmt.Errorf("failed to handle button: %w", err)
			}
			repeater.Start(key)
			return nil
		}
		repeater.Stop(key)
	}

	if err := inputHandler(InputEvent{Key: key, IsDown: isDown}); err != nil {
		return fmt.Errorf("failed to handle button: %w", err)
	}
	return nil
}

func handleJoystickAxisEvent(event *sdl.JoyAxisEvent, inputHandler func(InputEvent) error, lastDir *int, repeater *keyRepeater) error {
	if event.Axis != leftStickYAxis {
		return nil
	}

	dir := 0
	if event.Value <= -leftStickAxisThreshold {
		dir = -1
	} else if event.Value >= leftStickAxisThreshold {
		dir = 1
	}

	if dir == *lastDir {
		return nil
	}

	keyForDir := func(d int) int {
		if d < 0 {
			return protocol.KeyScrollUp
		}
		return protocol.KeyScrollDown
	}

	if *lastDir != 0 {
		oldKey := keyForDir(*lastDir)
		repeater.Stop(oldKey)
		if err := inputHandler(InputEvent{Key: oldKey, IsDown: false}); err != nil {
			return fmt.Errorf("failed to release axis scroll key: %w", err)
		}
	}

	if dir != 0 {
		newKey := keyForDir(dir)
		if err := inputHandler(InputEvent{Key: newKey, IsDown: true}); err != nil {
			return fmt.Errorf("failed to press axis scroll key: %w", err)
		}
		repeater.Start(newKey)
	}

	*lastDir = dir
	return nil
}
