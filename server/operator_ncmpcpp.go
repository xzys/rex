package server

import (
	"strconv"

	"github.com/xzys/TR3X/v2/protocol"
)

type NcmpcppOperator struct {
	screenActive int
}

func NewNcmpcppOperator() *NcmpcppOperator {
	return &NcmpcppOperator{
		screenActive: 0,
	}
}

func (o *NcmpcppOperator) HandleKey(key int) error {
	var err error
	switch key {
	case protocol.KeyA:
		err = execute("tmux", "send-keys", "-t", "0", "Enter") // select item
	case protocol.KeyB:
		err = execute("tmux", "send-keys", "-t", "0", "p") // play/pause
	case protocol.KeyX:
		err = execute("tmux", "send-keys", "-t", "0", ">") // skip item
	case protocol.KeyY:
		err = execute("tmux", "send-keys", "-t", "0", "<") // back
	}
	return err
}

func (o *NcmpcppOperator) nextScreen() error {
	o.screenActive = (o.screenActive + 1) % 8
	return execute("tmux", "send-keys", "-t", "0", strconv.Itoa(o.screenActive+1))
}
