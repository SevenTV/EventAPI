package client

import (
	"encoding/json"
	"strings"

	"github.com/SevenTV/EventAPI/src/global"
	"github.com/seventv/common/events"
)

func NewHandler(conn Connection) Handler {
	return Handler{conn}
}

type Handler struct {
	conn Connection
}

func (h Handler) Subscribe(gctx global.Context, m events.Message[json.RawMessage]) error {
	msg, err := events.ConvertMessage[events.SubscribePayload](m)
	if err != nil {
		return err
	}

	t := msg.Data.Type
	path := strings.Split(string(t), ".")

	// Empty subscription event type
	if t == "" {
		h.conn.SendError("Missing event type", nil)
		h.conn.Close(events.CloseCodeInvalidPayload)
		return nil
	}
	if len(path) < 2 {
		h.conn.SendError("Bad event type path", nil)
		h.conn.Close(events.CloseCodeInvalidPayload)
		return nil
	}

	// No targets: this requires authentication
	if len(msg.Data.Targets) == 0 && h.conn.Actor() == nil {
		h.conn.SendError("Wildcard event target subscription requires authentication", nil)
		h.conn.Close(events.CloseCodeInsufficientPrivilege)
		return nil
	}

	// Add the event subscription
	_, err = h.conn.Events().Subscribe(gctx, t, msg.Data.Targets)
	if err != nil {
		if err == ErrAlreadySubscribed {
			h.conn.Close(events.CloseCodeAlreadySubscribed)
			return nil
		}
		return err
	}
	return nil
}

func (h Handler) Unsubscribe(gctx global.Context, m events.Message[json.RawMessage]) error {
	msg, err := events.ConvertMessage[events.UnsubscribePayload](m)
	if err != nil {
		return err
	}

	t := msg.Data.Type
	if err = h.conn.Events().Unsubscribe(t); err != nil {
		if err == ErrNotSubscribed {
			h.conn.Close(events.CloseCodeNotSubscribed)
			return nil
		}
		return err
	}

	return nil
}
