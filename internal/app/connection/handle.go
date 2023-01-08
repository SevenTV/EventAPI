package client

import (
	"encoding/json"
	"errors"
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
	OnDispatch(gctx global.Context, msg events.Message[events.DispatchPayload]) bool
}

type handler struct {
	conn Connection
}

const (
	EVENT_TYPE_MAX_LENGTH                   = 64
	SUBSCRIPTION_CONDITION_MAX              = 10
	SUBSCRIPTION_CONDITION_KEY_MAX_LENGTH   = 64
	SUBSCRIPTION_CONDITION_VALUE_MAX_LENGTH = 128
)

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
		h.conn.Close(events.CloseCodeInvalidPayload, 0)

		return nil, false
	}
	if len(path) < 2 {
		h.conn.SendError("Bad event type path", nil)
		h.conn.Close(events.CloseCodeInvalidPayload, 0)

		return nil, false
	}

	// No targets: this requires authentication
	if len(msg.Data.Condition) == 0 && h.conn.Actor() == nil {
		h.conn.SendError("Wildcard event target subscription requires authentication", nil)
		h.conn.Close(events.CloseCodeInsufficientPrivilege, 0)

		return nil, false
	}

	// Too many subscriptions?
	if h.conn.Events().Count() >= gctx.Config().API.SubscriptionLimit {
		h.conn.SendError("Too Many Active Subscriptions!", nil)
		h.conn.Close(events.CloseCodeRateLimit, 0)

		return nil, false
	}

	// Validate: event type
	if len(msg.Data.Type) > EVENT_TYPE_MAX_LENGTH {
		h.conn.SendError("Event Type Too Large", map[string]any{
			"event_type":             msg.Data.Type,
			"event_type_length":      len(msg.Data.Type),
			"event_type_length_most": EVENT_TYPE_MAX_LENGTH,
		})
		h.conn.Close(events.CloseCodeRateLimit, 0)

		return nil, false
	}

	// Validate: condition
	pos := -1
	for k, v := range msg.Data.Condition {
		pos++

		if pos > SUBSCRIPTION_CONDITION_MAX {
			h.conn.SendError("Subscription Condition Too Large", map[string]any{
				"condition_keys":      len(msg.Data.Condition),
				"condition_keys_most": SUBSCRIPTION_CONDITION_MAX,
			})
			h.conn.Close(events.CloseCodeRateLimit, 0)

			return nil, false
		}

		kL := len(k)
		vL := len(v)

		if kL > SUBSCRIPTION_CONDITION_KEY_MAX_LENGTH || vL > SUBSCRIPTION_CONDITION_VALUE_MAX_LENGTH {
			h.conn.SendError("Subscription Condition Key Too Large", map[string]any{
				"key":               k,
				"key_index":         pos,
				"value":             v,
				"key_length":        kL,
				"key_length_most":   SUBSCRIPTION_CONDITION_KEY_MAX_LENGTH,
				"value_length":      vL,
				"value_length_most": SUBSCRIPTION_CONDITION_VALUE_MAX_LENGTH,
			})
			h.conn.Close(events.CloseCodeRateLimit, 0)

			return nil, false
		}
	}

	// Add the event subscription
	_, id, err := h.conn.Events().Subscribe(gctx, t, msg.Data.Condition)
	if err != nil {
		switch err {
		case ErrAlreadySubscribed:
			h.conn.SendError("Already subscribed to this event", nil)
			h.conn.Close(events.CloseCodeAlreadySubscribed, 0)

			return nil, false
		default:
			return err, false
		}
	}

	_ = h.conn.SendAck(events.OpcodeSubscribe, utils.ToJSON(struct {
		ID        uint32            `json:"id"`
		Type      string            `json:"type"`
		Condition map[string]string `json:"condition"`
	}{
		ID:        id,
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
	if _, err = h.conn.Events().Unsubscribe(t, msg.Data.Condition); err != nil {
		if err == ErrNotSubscribed {
			h.conn.Close(events.CloseCodeNotSubscribed, 0)
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

func (h handler) OnDispatch(gctx global.Context, msg events.Message[events.DispatchPayload]) bool {
	// Filter by subscribed event types
	ev, ok := h.conn.Events().Get(msg.Data.Type)
	if !ok {
		return false // skip if not subscribed to this
	}

	matches, ok := ev.Match(msg.Data.Conditions)
	if !ok {
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

	// Handle effect
	if msg.Data.Effect != nil {
		for _, e := range msg.Data.Effect.AddSubscriptions {
			_, _, err := h.conn.Events().Subscribe(gctx, e.Type, e.Condition)
			if err != nil && !errors.Is(err, ErrAlreadySubscribed) {
				zap.S().Errorw("failed to add subscription from dispatch",
					"error", err,
				)
			}
		}

		for _, e := range msg.Data.Effect.RemoveSubscriptions {
			_, err := h.conn.Events().Unsubscribe(e.Type, e.Condition)
			if err != nil && !errors.Is(err, ErrNotSubscribed) {
				zap.S().Errorw("failed to remove subscription from dispatch",
					"error", err,
				)
			}
		}

		for _, ha := range msg.Data.Effect.RemoveHashes {
			h.conn.Cache().ExpireDispatch(ha)
		}
	}

	msg.Data.Conditions = nil
	msg.Data.Effect = nil
	msg.Data.Matches = matches

	if err := h.conn.Write(msg.ToRaw()); err != nil {
		zap.S().Errorw("failed to write dispatch to connection",
			"error", err,
		)

		return false
	}

	return true
}
