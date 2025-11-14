//go:build !sdl
// +build !sdl

package server

func (h *InputHandler) Initialize() error {
	return nil
}
