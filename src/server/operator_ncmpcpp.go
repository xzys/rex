package server

import (
	"strconv"

	"github.com/xrfr/t-rex/v2/protocol"
)

type NcmpcppOperator struct {
	screenActive int
}

func NewNcmpcppOperator() *NcmpcppOperator {
	return &NcmpcppOperator{
		screenActive: 0,
	}
}

func (o *NcmpcppOperator) HandleInput(key int, isDown bool) (bool, error) {
	if !isDown {
		return false, nil
	}
	var err error
	switch key {
	case protocol.KeyDPadRight:
		err = o.nextScreen()
	case protocol.KeyDPadLeft:
		err = o.prevScreen()
	case protocol.KeyB:
		err = execute("tmux", "send-keys", "-t", "0", "p") // play/pause
	case protocol.KeyX:
		err = execute("tmux", "send-keys", "-t", "0", ">") // skip item
	case protocol.KeyY:
		err = execute("tmux", "send-keys", "-t", "0", "<") // back
	default:
		return false, nil
	}
	return true, err
}

func (o *NcmpcppOperator) nextScreen() error {
	o.screenActive = (o.screenActive + 1) % 8
	return execute("tmux", "send-keys", "-t", "0", strconv.Itoa(o.screenActive+1))
}

func (o *NcmpcppOperator) prevScreen() error {
	o.screenActive = (o.screenActive - 1 + 8) % 8
	return execute("tmux", "send-keys", "-t", "0", strconv.Itoa(o.screenActive+1))
}
