package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/seventv/api/data/events"
	"github.com/seventv/common/utils"
	"github.com/seventv/eventapi/internal/global"
	"go.uber.org/zap"
)

// EventBuffer handles the buffering of events
// and subscriptions in the event a connection drops
//
// This allows a grace period where a client may resume their session
// and recover missed events and subscriptions
type EventBuffer interface {
	Context() context.Context
	// Start begins tracking events and subscriptions
	Start(gctx global.Context) error
	// Push messages to the buffer
	Push(gctx global.Context, msg events.Message[events.DispatchPayload]) error
	// Recover retrieves the buffer from the previous session
	Recover(gctx global.Context) (eventList []events.Message[events.DispatchPayload], subList []StoredSubscription, err error)
	// Cleanup clears out redis keys
	Cleanup(gctx global.Context) error
}

type eventBuffer struct {
	ctx    context.Context
	cancel context.CancelFunc

	// the connection that this buffer is associated with
	conn          Connection
	stateKey      string
	eventStoreKey string
	subStoreKey   string

	ttl time.Time
}

func NewEventBuffer(conn Connection, sessionID string, ttl time.Duration) EventBuffer {
	ttlAt := time.Now().Add(ttl)

	ctx, cancel := context.WithTimeout(context.Background(), ttl)

	return &eventBuffer{
		ctx:           ctx,
		cancel:        cancel,
		conn:          conn,
		stateKey:      fmt.Sprintf("events:session:%s:recovery", sessionID),
		eventStoreKey: fmt.Sprintf("events:session:%s:event_buffer", sessionID),
		subStoreKey:   fmt.Sprintf("events:session:%s:sub_buffer", sessionID),
		ttl:           ttlAt,
	}
}

func (b *eventBuffer) Context() context.Context {
	return b.ctx
}

func (b *eventBuffer) Start(gctx global.Context) error {
	// Define session as recoverable
	pipe := gctx.Inst().Redis.RawClient().Pipeline()

	pipe.Set(b.ctx, b.stateKey, "1", time.Until(b.ttl))

	// Store session's subscriptions
	b.conn.Events().m.Range(func(key events.EventType, value EventChannel) bool {
		sub, err := json.Marshal(StoredSubscription{
			Type:    key,
			Channel: value,
		})
		if err != nil {
			zap.S().Errorw("failed to marshal subscription for buffered storage", "error", err)

			return false
		}

		pipe.LPush(b.ctx, b.subStoreKey, utils.B2S(sub))

		return true
	})

	pipe.ExpireAt(b.ctx, b.subStoreKey, b.ttl)

	if _, err := pipe.Exec(b.ctx); err != nil {
		return err
	}

	return nil
}

func (b *eventBuffer) Recover(gctx global.Context) (eventList []events.Message[events.DispatchPayload], subList []StoredSubscription, err error) {
	// check if session is recoverable
	if _, err = gctx.Inst().Redis.RawClient().Get(b.ctx, b.stateKey).Result(); err != nil {
		return nil, nil, ErrNotRecoverable
	}

	// recover events
	for {
		s, _ := gctx.Inst().Redis.RawClient().LPopCount(b.ctx, b.eventStoreKey, 100).Result()
		if len(s) == 0 {
			break
		}

		for _, v := range s {
			var msg events.Message[events.DispatchPayload]
			if err = json.Unmarshal(utils.S2B(v), &msg); err != nil {
				break
			}

			eventList = append(eventList, msg)
		}
	}

	// recover subscriptions
	for {
		s, _ := gctx.Inst().Redis.RawClient().LPopCount(b.ctx, b.subStoreKey, 100).Result()
		if len(s) == 0 {
			break
		}

		for _, v := range s {
			var sub StoredSubscription
			if err = json.Unmarshal(utils.S2B(v), &sub); err != nil {
				break
			}

			subList = append(subList, sub)
		}
	}

	return eventList, subList, err
}

func (b *eventBuffer) Push(gctx global.Context, msg events.Message[events.DispatchPayload]) error {
	if b.ctx.Err() != nil {
		return ErrBufferClosed
	}

	s, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	pipe := gctx.Inst().Redis.RawClient().Pipeline()

	// push the event to the redis list
	pipe.LPush(b.ctx, b.eventStoreKey, utils.B2S(s))

	// set the TTL on the key to expire at the buffer's expire
	pipe.ExpireAt(b.ctx, b.eventStoreKey, b.ttl)

	_, err = pipe.Exec(b.ctx)

	return err
}

func (b *eventBuffer) Cleanup(gctx global.Context) error {
	pipe := gctx.Inst().Redis.RawClient().Pipeline()

	for _, key := range []string{b.stateKey, b.eventStoreKey, b.subStoreKey} {
		pipe.Del(b.ctx, key)
	}

	_, err := pipe.Exec(b.ctx)

	return err
}

type StoredSubscription struct {
	Type    events.EventType `json:"type"`
	Channel EventChannel     `json:"channel"`
}

var (
	ErrBufferClosed   = errors.New("event buffer closed")
	ErrNotRecoverable = errors.New("session is not recoverable")
)
