//go:build !sdl
// +build !sdl

package server

func (s *Server) Initialize() error {
	return nil
}
