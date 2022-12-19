package client

import (
	"encoding/json"
	"strings"

	"github.com/seventv/api/data/events"
	"github.com/seventv/common/utils"
	"github.com/seventv/eventapi/internal/global"
	"go.uber.org/zap"
)

func NewHandler(conn Connection) Handler {
	return handler{conn}
}

type Handler interface {
	Subscribe(gctx global.Context, m events.Message[json.RawMessage]) (error, bool)
	Unsubscribe(gctx global.Context, m events.Message[json.RawMessage]) error
	OnDispatch(msg events.Message[events.DispatchPayload]) bool
}

type handler struct {
	conn Connection
}

func (h handler) Subscribe(gctx global.Context, m events.Message[json.RawMessage]) (error, bool) {
	msg, err := events.ConvertMessage[events.SubscribePayload](m)
	if err != nil {
		return err, false
	}

	t := msg.Data.Type
	path := strings.Split(string(t), ".")

	// Empty subscription event type
	if t == "" {
		h.conn.SendError("Missing event type", nil)
		h.conn.Close(events.CloseCodeInvalidPayload)

		return nil, false
	}
	if len(path) < 2 {
		h.conn.SendError("Bad event type path", nil)
		h.conn.Close(events.CloseCodeInvalidPayload)

		return nil, false
	}

	// No targets: this requires authentication
	if len(msg.Data.Condition) == 0 && h.conn.Actor() == nil {
		h.conn.SendError("Wildcard event target subscription requires authentication", nil)
		h.conn.Close(events.CloseCodeInsufficientPrivilege)

		return nil, false
	}

	// Too many subscriptions?
	if h.conn.Events().Count() >= gctx.Config().API.SubscriptionLimit {
		h.conn.SendError("Too Many Active Subscriptions!", nil)
		h.conn.Close(events.CloseCodeRateLimit)

		return nil, false
	}

	// Add the event subscription
	_, err = h.conn.Events().Subscribe(gctx, t, msg.Data.Condition)
	if err != nil {
		if err == ErrAlreadySubscribed {
			h.conn.Close(events.CloseCodeAlreadySubscribed)

			return nil, false
		}

		return err, false
	}

	_ = h.conn.SendAck(events.OpcodeSubscribe, utils.ToJSON(struct {
		Type      string            `json:"type"`
		Condition map[string]string `json:"condition"`
	}{
		Type:      string(msg.Data.Type),
		Condition: msg.Data.Condition,
	}))

	return nil, true
}

func (h handler) Unsubscribe(gctx global.Context, m events.Message[json.RawMessage]) error {
	msg, err := events.ConvertMessage[events.UnsubscribePayload](m)
	if err != nil {
		return err
	}

	t := msg.Data.Type
	if err = h.conn.Events().Unsubscribe(t, msg.Data.Condition); err != nil {
		if err == ErrNotSubscribed {
			h.conn.Close(events.CloseCodeNotSubscribed)
			return nil
		}
		return err
	}

	_ = h.conn.SendAck(events.OpcodeSubscribe, utils.ToJSON(struct {
		Type      string            `json:"type"`
		Condition map[string]string `json:"condition"`
	}{
		Type:      string(msg.Data.Type),
		Condition: msg.Data.Condition,
	}))

	return nil
}

func (h handler) OnDispatch(msg events.Message[events.DispatchPayload]) bool {
	// Filter by subscribed event types
	ev, ok := h.conn.Events().Get(msg.Data.Type)
	if !ok {
		return false // skip if not subscribed to this
	}

	if !ev.Match(msg.Data.Conditions) {
		return false
	}

	// Dedupe
	if msg.Data.Hash != nil {
		ha := *msg.Data.Hash

		if !h.conn.Cache().AddDispatch(ha) {
			return false // skip if already dispatched
		}

		msg.Data.Hash = nil
	}

	msg.Data.Conditions = nil

	if err := h.conn.Write(msg.ToRaw()); err != nil {
		zap.S().Errorw("failed to write dispatch to connection",
			"error", err,
		)

		return false
	}

	return true
}
