//go:build !sdl
// +build !sdl

package server

func (s *Server) Initialize() error {
	// in keyboard mode, keyboard app handles inputs
	return nil
}
