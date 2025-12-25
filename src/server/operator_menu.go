package server

import "github.com/xrfr/t-rex/v2/protocol"

type MenuOperator struct{}

func NewMenuOperator() *MenuOperator {
	return &MenuOperator{}
}

func (o *MenuOperator) HandleKey(key int) (bool, error) {
	switch key {
	case protocol.KeyA:
		return true, execute("tmux", "send-keys", "-t", "0", "Enter")
	}
	return false, nil
}
