//go:build !sdl
// +build !sdl

package server

import (
	"log/slog"
	"os"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/xrfr/t-rex/v2/protocol"
)

type KeyboardInputDevice struct{}

func init() {
	inputDevice = &KeyboardInputDevice{}
}

func (d *KeyboardInputDevice) Initialize(handler func(InputEvent) error) error {
	go runKeyboardInputLoop(handler)
	return nil
}

func runKeyboardInputLoop(handler func(InputEvent) error) {
	// handle errors and exit
	var err error
	defer func() {
		if err != nil {
			slog.Error("error during keyboard input loop", "err", err)
			os.Exit(1)
		} else {
			os.Exit(0)
		}
	}()

	if err = keyboard.Open(); err != nil {
		slog.Error("failed to open keyboard listener", "err", err)
		return
	}
	defer keyboard.Close()

	err = func() error {
		slog.Info("listening for keyboard input (press Ctrl+C to stop)")
		var lastKey int = -1
		var lastTime time.Time
		const debounce = 50 * time.Millisecond

		for {
			char, key, err := keyboard.GetKey()
			slog.Debug("got key", "char", char, "key", key)
			if err != nil {
				return err
			}

			if key == keyboard.KeyCtrlC {
				slog.Info("Ctrl+C pressed, stopping keyboard input loop")
				return nil
			}

			mappedKey := handleKeyPress(char, key)
			if mappedKey == -1 {
				continue
			}

			// Debounce: ignore repeated same key within threshold (OS key repeat)
			now := time.Now()
			if mappedKey == lastKey && now.Sub(lastTime) < debounce {
				continue
			}
			lastKey = mappedKey
			lastTime = now

			event := InputEvent{Key: mappedKey, IsDown: true}
			if err := handler(event); err != nil {
				return err
			}
		}
	}()
}

func handleKeyPress(char rune, key keyboard.Key) int {
	switch key {
	case keyboard.KeyArrowUp:
		return protocol.KeyDPadUp
	case keyboard.KeyArrowDown:
		return protocol.KeyDPadDown
	case keyboard.KeyArrowLeft:
		return protocol.KeyDPadLeft
	case keyboard.KeyArrowRight:
		return protocol.KeyDPadRight
	}

	switch char {
	// WASD for D-Pad (in addition to arrow keys above)
	case 'w':
		return protocol.KeyDPadUp
	case 'a':
		return protocol.KeyDPadLeft
	case 's':
		return protocol.KeyDPadDown
	case 'd':
		return protocol.KeyDPadRight
	// Right-hand face buttons
	case 'm':
		return protocol.KeyY
	case ',':
		return protocol.KeyX
	case '.':
		return protocol.KeyB
	case '/':
		return protocol.KeyA
	// Shoulder buttons
	case 'z':
		return protocol.KeyL1
	case 'x':
		return protocol.KeyR1
	case 'c':
		return protocol.KeyL2
	case 'v':
		return protocol.KeyR2
	default:
		return -1
	}
}
