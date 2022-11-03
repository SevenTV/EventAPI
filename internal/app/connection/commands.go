package client

import (
	"encoding/json"
	"strings"

	"github.com/seventv/api/data/events"
	"github.com/seventv/eventapi/internal/global"
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
		h.conn.Close(events.CloseCodeInvalidPayload, true)
		return nil
	}
	if len(path) < 2 {
		h.conn.SendError("Bad event type path", nil)
		h.conn.Close(events.CloseCodeInvalidPayload, true)
		return nil
	}

	// No targets: this requires authentication
	if len(msg.Data.Condition) == 0 && h.conn.Actor() == nil {
		h.conn.SendError("Wildcard event target subscription requires authentication", nil)
		h.conn.Close(events.CloseCodeInsufficientPrivilege, true)
		return nil
	}

	// Too many subscriptions?
	if h.conn.Events().Count() >= gctx.Config().API.ConnectionLimit {
		h.conn.SendError("Too Many Active Subscriptions!", nil)
		h.conn.Close(events.CloseCodeRateLimit, true)
	}

	// Add the event subscription
	_, err = h.conn.Events().Subscribe(gctx, t, msg.Data.Condition)
	if err != nil {
		if err == ErrAlreadySubscribed {
			h.conn.Close(events.CloseCodeAlreadySubscribed, true)
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
	if err = h.conn.Events().Unsubscribe(t, msg.Data.Condition); err != nil {
		if err == ErrNotSubscribed {
			h.conn.Close(events.CloseCodeNotSubscribed, true)
			return nil
		}
		return err
	}

	return nil
}
