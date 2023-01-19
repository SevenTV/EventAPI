package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/seventv/api/data/events"
	"github.com/seventv/common/utils"
	"github.com/seventv/eventapi/internal/global"
)

// EventBuffer handles the buffering of events
// and subscriptions in the event a connection drops
//
// This allows a grace period where a client may resume their session
// and recover missed events and subscriptions
type EventBuffer interface {
	// Start begins tracking events and subscriptions
	Start(gctx global.Context) error
	// Recover retrieves the buffer from the previous session
	Recover(gctx global.Context) (eventList []events.Message[events.DispatchPayload], subList []StoredSubscription, err error)
	Push(gctx global.Context, msg events.Message[events.DispatchPayload]) error

	Done() <-chan time.Time
}

type eventBuffer struct {
	// the connection that this buffer is associated with
	conn          Connection
	stateKey      string
	eventStoreKey string
	subStoreKey   string

	ttl time.Time

	// whether or not the buffer is open
	open bool
}

func NewEventBuffer(conn Connection, sessionID string, ttl time.Duration) EventBuffer {
	return &eventBuffer{
		conn:          conn,
		stateKey:      fmt.Sprintf("events:session:%s:recovery", sessionID),
		eventStoreKey: fmt.Sprintf("events:session:%s:event_buffer", sessionID),
		subStoreKey:   fmt.Sprintf("events:session:%s:sub_buffer", sessionID),
		ttl:           time.Now().Add(ttl),
	}
}

func (b *eventBuffer) Start(gctx global.Context) error {
	b.open = true

	// Define session as recoverable
	if _, err := gctx.Inst().Redis.RawClient().Set(b.conn.Context(), b.stateKey, "1", time.Until(b.ttl)).Result(); err != nil {
		return err
	}

	return nil
}

func (b *eventBuffer) Recover(gctx global.Context) (eventList []events.Message[events.DispatchPayload], subList []StoredSubscription, err error) {
	// check if session is recoverable
	if _, err = gctx.Inst().Redis.RawClient().Get(b.conn.Context(), b.stateKey).Result(); err != nil {
		return nil, nil, ErrNotRecoverable
	}

	// recover events
	for {
		s, _ := gctx.Inst().Redis.RawClient().LPopCount(b.conn.Context(), b.eventStoreKey, 100).Result()
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
		s, _ := gctx.Inst().Redis.RawClient().LPopCount(b.conn.Context(), b.subStoreKey, 100).Result()
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
	if !b.open {
		return ErrBufferClosed
	}

	s, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// push the event to the redis list
	if _, err = gctx.Inst().Redis.RawClient().LPush(b.conn.Context(), b.eventStoreKey, utils.B2S(s)).Result(); err != nil {
		return err
	}

	// set the TTL on the key to expire at the buffer's expire
	if _, err = gctx.Inst().Redis.RawClient().ExpireAt(b.conn.Context(), b.eventStoreKey, b.ttl).Result(); err != nil {
		return err
	}

	return nil
}

func (b *eventBuffer) Done() <-chan time.Time {
	return time.After(time.Until(b.ttl))
}

type StoredSubscription struct {
	Type    events.EventType
	Channel EventChannel
}

var (
	ErrBufferClosed   = errors.New("event buffer closed")
	ErrNotRecoverable = errors.New("session is not recoverable")
)
