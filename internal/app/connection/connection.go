package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/seventv/api/data/events"
	"github.com/seventv/common/structures/v3"
	"github.com/seventv/common/utils"

	"github.com/seventv/eventapi/internal/global"
	"github.com/seventv/eventapi/internal/nats"
)

type Connection interface {
	Context() context.Context
	// Retrieve the hex-encoded ID of this session
	SessionID() string
	// Greet sends an Hello message to the client
	Greet(gctx global.Context) error
	// Listen for incoming and outgoing events
	Read(gctx global.Context)
	// SendHeartbeat lets the client know that the connection is healthy
	SendHeartbeat() error
	// SendAck sends an Ack message to the client
	SendAck(cmd events.Opcode, data json.RawMessage) error
	// SendError publishes an error message to the client
	SendError(txt string, fields map[string]any)
	// Write sends a message to the client
	Write(msg events.Message[json.RawMessage]) error
	// Actor returns the authenticated user for this connection
	Actor() *structures.User
	// Handler returns a utility to handle commands for the connection
	Handler() Handler
	// Subscriptions returns an instance of Events
	Events() *EventMap
	// Cache returns the connection's cache utility
	Cache() Cache
	// Buffer returns the connection's event buffer utility for resuming the session
	Buffer() EventBuffer
	// Ready returns a channel that is closed when the connection is ready
	OnReady() <-chan struct{}
	// OnClose returns a channel that is closed when the connection is closed
	OnClose() <-chan struct{}
	// Close sends a close frame with the specified code and ends the connection
	SendClose(code events.CloseCode, after time.Duration)
	// SetWriter defines the connection's writable stream (SSE only)
	SetWriter(w *bufio.Writer, f http.Flusher)
	// Return the name of the transport used by this connection
	Transport() Transport
}

func IsClientSentOp(op events.Opcode) bool {
	switch op {
	case events.OpcodeHeartbeat,
		events.OpcodeIdentify,
		events.OpcodeResume,
		events.OpcodeSubscribe,
		events.OpcodeUnsubscribe,
		events.OpcodeSignal,
		events.OpcodeBridge:
		return true
	default:
		return false
	}
}

func GenerateSessionID(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func NewEventMap(sessionID string) *EventMap {
	return &EventMap{
		subscription: nats.NewSub(sessionID),
		count:        utils.PointerOf(int32(0)),
		m:            map[events.EventType]EventChannel{},
		mx:           sync.Mutex{},
	}
}

type EventMap struct {
	subscription *nats.Subscription
	count        *int32
	m            map[events.EventType]EventChannel
	mx           sync.Mutex
	once         sync.Once
}

// Subscribe sets up a subscription to dispatch events with the specified type
func (e *EventMap) Subscribe(
	gctx global.Context,
	ctx context.Context,
	t events.EventType,
	cond events.EventCondition,
	props EventSubscriptionProperties,
) (EventChannel, uint32, error) {
	e.mx.Lock()
	defer e.mx.Unlock()

	id := rand.Uint32()

	ec, exists := e.m[t]
	if !exists {
		lctx, cancel := context.WithCancel(ctx)

		ec = EventChannel{
			ctx:        lctx,
			cancel:     cancel,
			ID:         []uint32{},
			Conditions: []events.EventCondition{},
			Properties: []EventSubscriptionProperties{},
		}
	}

	if exists && len(ec.Conditions) == 0 && len(cond) == 0 {
		return ec, id, ErrAlreadySubscribed
	}

	for i, c := range ec.Conditions {
		if c.Match(cond) {
			if ec.Properties[i].Auto {
				return ec, id, nil
			}

			return ec, id, ErrAlreadySubscribed
		}
	}

	ec.ID = append(ec.ID, id)
	ec.Conditions = append(ec.Conditions, cond)
	ec.Properties = append(ec.Properties, props)

	// Create channel
	e.m[t] = ec

	e.subscription.Subscribe(events.CreateDispatchKey(t, cond, false))

	return ec, id, nil
}

func (e *EventMap) Unsubscribe(gctx global.Context, t events.EventType, cond map[string]string) (uint32, error) {
	e.mx.Lock()
	defer e.mx.Unlock()

	if len(cond) == 0 {
		_, exists := e.m[t]
		delete(e.m, t)
		if !exists {
			return 0, ErrNotSubscribed
		}

		return 0, nil
	}

	ec, exists := e.m[t]
	if !exists {
		return 0, ErrNotSubscribed
	}

	var id uint32

	matched := false

	for i, c := range ec.Conditions {
		if c.Match(cond) {
			id = ec.ID[i]

			ec.ID = utils.SliceRemove(ec.ID, i)
			ec.Conditions = utils.SliceRemove(ec.Conditions, i)
			ec.Properties = utils.SliceRemove(ec.Properties, i)

			e.subscription.Unsubscribe(events.CreateDispatchKey(t, c, false))

			matched = true
			break
		}
	}

	if !matched {
		return 0, ErrNotSubscribed
	}

	if len(ec.ID) == 0 {
		ec.cancel()
		delete(e.m, t)
	} else {
		e.m[t] = ec
	}

	return id, nil
}

func (e *EventMap) UnsubscribeWithID(id ...uint32) error {
	var found bool
	e.mx.Lock()
	defer e.mx.Unlock()

outer:
	for t, value := range e.m {
		for _, id := range id {
			for i, v := range value.ID {
				if v == id {
					e.subscription.Unsubscribe(events.CreateDispatchKey(t, value.Conditions[i], false))

					value.ID = utils.SliceRemove(value.ID, i)
					value.Conditions = utils.SliceRemove(value.Conditions, i)
					value.Properties = utils.SliceRemove(value.Properties, i)

					found = true
					break outer
				}
			}
		}
	}

	if !found {
		return ErrNotSubscribed
	}

	return nil
}

func (e *EventMap) Count() int32 {
	return *e.count
}

func (e *EventMap) Get(t events.EventType) (*EventChannel, bool) {
	e.mx.Lock()
	defer e.mx.Unlock()

	var ec *EventChannel

	if c, ok := e.m[t]; ok {
		ec = &c
	}

	tWilcard := events.EventType(fmt.Sprintf("%s.*", t.ObjectName()))
	if c, ok := e.m[tWilcard]; ok {
		if ec == nil {
			ec = &EventChannel{
				ID:         []uint32{},
				Conditions: []events.EventCondition{},
				Properties: []EventSubscriptionProperties{},
			}
		}

		ec.ID = append(ec.ID, c.ID...)
		ec.Conditions = append(ec.Conditions, c.Conditions...)
		ec.Properties = append(ec.Properties, c.Properties...)
	}

	return ec, ec != nil
}

func (e *EventMap) DispatchChannel() chan []byte {
	return e.subscription.Ch
}

func (e *EventMap) Destroy(gctx global.Context) {
	e.once.Do(func() {
		e.mx.Lock()
		defer e.mx.Unlock()

		for key, value := range e.m {
			value.cancel()
			delete(e.m, key)
		}

		e.subscription.Close()
	})
}

type EventChannel struct {
	ctx        context.Context
	cancel     context.CancelFunc
	ID         []uint32                      `json:"id"`
	Conditions []events.EventCondition       `json:"conditions"`
	Properties []EventSubscriptionProperties `json:"properties"`
}

type EventSubscriptionProperties struct {
	TTL  time.Time
	Auto bool
}

func (ec EventChannel) Match(cond []events.EventCondition) []uint32 {
	if len(ec.Conditions) == 0 { // No condition
		return ec.ID
	}

	matches := make(utils.Set[uint32], 0)

	for _, c := range cond {
		for i, e := range ec.Conditions {
			if e.Match(c) && len(ec.ID) >= i {
				matches.Add(ec.ID[i])
			}
		}
	}

	return matches.Values()
}

var (
	ErrAlreadySubscribed = fmt.Errorf("already subscribed")
	ErrNotSubscribed     = fmt.Errorf("not subscribed")
)

type Transport string

const (
	TransportWebSocket   Transport = "WebSocket"
	TransportEventStream Transport = "EventStream"
)
