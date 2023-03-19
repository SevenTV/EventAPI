package client

import "github.com/seventv/eventapi/internal/global"

type EventReceiver interface {
}

type eventReceiver struct {
}

func NewEventReceiver(gctx global.Context) EventReceiver {
	return &eventReceiver{}
}
