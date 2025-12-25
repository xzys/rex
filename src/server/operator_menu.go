package server

import "github.com/xrfr/t-rex/v2/protocol"

type MenuOperator struct{}

func NewMenuOperator() *MenuOperator {
	return &MenuOperator{}
}

func (o *MenuOperator) HandleKey(key int) error {
	switch key {
	case protocol.KeyA:
		return execute("tmux", "send-keys", "-t", "0", "Enter")
	}
	return nil
}

