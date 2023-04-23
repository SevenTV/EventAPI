package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

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
	OnDispatch(gctx global.Context, msg events.Message[events.DispatchPayload])
	OnResume(gctx global.Context, msg events.Message[json.RawMessage]) error
	OnBridge(gctx global.Context, msg events.Message[json.RawMessage]) error
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

func (h handler) OnDispatch(gctx global.Context, msg events.Message[events.DispatchPayload]) {
	var matches []uint32

	if msg.Data.Whisper == "" {
		// Filter by subscribed event types
		ev, ok := h.conn.Events().Get(msg.Data.Type)
		if !ok {
			return // skip if not subscribed to this
		}

		matches = ev.Match(msg.Data.Conditions)
		if len(matches) == 0 {
			return
		}
	} else if msg.Data.Whisper != h.conn.SessionID() {
		return // skip if event is whisper not for this session
	}

	// Dedupe
	if msg.Data.Hash != nil {
		ha := *msg.Data.Hash

		if !h.conn.Cache().AddDispatch(ha) {
			return // skip if already dispatched
		}
	}

	// Handle effect
	if msg.Data.Effect != nil {
		for _, e := range msg.Data.Effect.AddSubscriptions {
			_, ids, err := h.conn.Events().Subscribe(gctx, h.conn.Context(), e.Type, e.Condition, EventSubscriptionProperties{
				TTL:  utils.Ternary(e.TTL > 0, time.Now().Add(e.TTL), time.Time{}),
				Auto: true,
			})
			if err != nil && !errors.Is(err, ErrAlreadySubscribed) {
				zap.S().Errorw("failed to add subscription from dispatch",
					"error", err,
				)
			}

			// Handle TTL: remove the subscription after TTL
			if e.TTL > 0 {
				go func(ttl time.Duration, typ events.EventType, cond events.EventCondition) {
					select {
					case <-h.conn.Context().Done():
						return
					case <-time.After(ttl):
					}

					err = h.conn.Events().UnsubscribeWithID(ids)
					if err != nil && !errors.Is(err, ErrNotSubscribed) {
						zap.S().Errorw("failed to remove subscription from dispatch after TTL expire",
							"error", err,
							"ttl", ttl.Milliseconds(),
							"type", typ,
							"condition", cond,
						)
					}
				}(e.TTL, e.Type, e.Condition)
			}
		}

		for _, e := range msg.Data.Effect.RemoveSubscriptions {
			_, err := h.conn.Events().Unsubscribe(gctx, e.Type, e.Condition)
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

	// connections with a buffer are dead connections where dispatches
	// are being saved to allow for a graceful recovery via resuming
	if h.conn.Buffer() != nil {
		if err := h.conn.Buffer().Push(gctx, msg); err != nil {
			zap.S().Errorw("failed to push dispatch to buffer",
				"error", err,
			)

			return
		}

		return
	}

	msg.Data.Conditions = nil
	msg.Data.Effect = nil
	msg.Data.Hash = nil
	msg.Data.Whisper = ""
	msg.Data.Matches = matches

	if err := h.conn.Write(msg.ToRaw()); err != nil {
		zap.S().Errorw("failed to write dispatch to connection",
			"error", err,
		)
	}
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
		h.conn.SendClose(events.CloseCodeInvalidPayload, 0)

		return nil, false
	}
	if len(path) < 2 {
		h.conn.SendError("Bad event type path", nil)
		h.conn.SendClose(events.CloseCodeInvalidPayload, 0)

		return nil, false
	}

	// No targets: this requires authentication
	if len(msg.Data.Condition) == 0 && h.conn.Actor() == nil {
		h.conn.SendError("Wildcard event target subscription requires authentication", nil)
		h.conn.SendClose(events.CloseCodeInsufficientPrivilege, 0)

		return nil, false
	}

	// Too many subscriptions?
	if h.conn.Events().Count() >= gctx.Config().API.SubscriptionLimit {
		h.conn.SendError("Too Many Active Subscriptions!", nil)
		h.conn.SendClose(events.CloseCodeRateLimit, 0)

		return nil, false
	}

	// Validate: event type
	if len(msg.Data.Type) > EVENT_TYPE_MAX_LENGTH {
		h.conn.SendError("Event Type Too Large", map[string]any{
			"event_type":             msg.Data.Type,
			"event_type_length":      len(msg.Data.Type),
			"event_type_length_most": EVENT_TYPE_MAX_LENGTH,
		})
		h.conn.SendClose(events.CloseCodeRateLimit, 0)

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
			h.conn.SendClose(events.CloseCodeRateLimit, 0)

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
			h.conn.SendClose(events.CloseCodeRateLimit, 0)

			return nil, false
		}
	}

	// Add the event subscription
	_, id, err := h.conn.Events().Subscribe(gctx, h.conn.Context(), t, msg.Data.Condition, EventSubscriptionProperties{})
	if err != nil {
		switch err {
		case ErrAlreadySubscribed:
			h.conn.SendError("Already subscribed to this event", nil)
			h.conn.SendClose(events.CloseCodeAlreadySubscribed, 0)

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
	if _, err = h.conn.Events().Unsubscribe(gctx, t, msg.Data.Condition); err != nil {
		if err == ErrNotSubscribed {
			h.conn.SendClose(events.CloseCodeNotSubscribed, 0)
			return nil
		}

		return err
	}

	_ = h.conn.SendAck(events.OpcodeUnsubscribe, utils.ToJSON(struct {
		Type      string            `json:"type"`
		Condition map[string]string `json:"condition"`
	}{
		Type:      string(msg.Data.Type),
		Condition: msg.Data.Condition,
	}))

	return nil
}

func (h handler) OnResume(gctx global.Context, m events.Message[json.RawMessage]) error {
	/*
		msg, err := events.ConvertMessage[events.ResumePayload](m)
		if err != nil {
			return err
		}

			// Set up a new event buffer with the specified session ID
			buf := NewEventBuffer(h.conn, msg.Data.SessionID, time.Minute)

			messages, subs, err := buf.Recover(gctx)
			subCount := 0

			if err == nil {
				// Reinstate subscriptions
				for _, s := range subs {
					for i := range s.Channel.ID {
						cond := s.Channel.Conditions[i]
						props := s.Channel.Properties[i]

						_, _, err := h.conn.Events().Subscribe(gctx, h.conn.Context(), s.Type, cond, props)
						if err != nil {
							return err
						}

						subCount++
					}
				}

				// Replay dispatches
				for _, m := range messages {
					h.OnDispatch(gctx, m)
				}
			} else {
				h.conn.SendError("Resume Failed", map[string]any{
					"error": err.Error(),
				})
			}*/

	// Send ACK
	_ = h.conn.SendAck(events.OpcodeResume, utils.ToJSON(struct {
		Success               bool `json:"success"`
		DispatchesReplayed    int  `json:"dispatches_replayed"`
		SubscriptionsRestored int  `json:"subscriptions_restored"`
	}{
		Success:               false,
		DispatchesReplayed:    0,
		SubscriptionsRestored: 0,
	}))

	/*
		// Cleanup the redis data
		if err = buf.Cleanup(gctx); err != nil {
			zap.S().Errorw("failed to cleanup event buffer", "error", err)
		}
	*/

	return nil
}

func (h handler) OnBridge(gctx global.Context, m events.Message[json.RawMessage]) error {
	msg, err := events.ConvertMessage[events.BridgedCommandPayload[json.RawMessage]](m)
	if err != nil {
		return err
	}

	msg.Data.SessionID = h.conn.SessionID()

	b, err := json.Marshal(msg.Data)
	if err != nil {
		return err
	}

	_, err = http.DefaultClient.Post(gctx.Config().API.BridgeURL, "application/json", bytes.NewReader(b))
	if err != nil {
		zap.S().Errorw("failed to bridge event", "error", err)

		return err
	}

	return nil
}
