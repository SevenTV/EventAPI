package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/seventv/api/data/events"
	"github.com/seventv/common/structures/v3"
	"github.com/seventv/common/sync_map"
	"github.com/seventv/common/utils"
	"github.com/seventv/eventapi/internal/global"
)

type Connection interface {
	Context() context.Context
	// Retrieve the hex-encoded ID of this session
	SessionID() string
	// Greet sends an Hello message to the client
	Greet() error
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
	Events() EventMap
	// Cache returns the connection's cache utility
	Cache() Cache
	// Ready returns a channel that is closed when the connection is ready
	OnReady() <-chan struct{}
	// OnClose returns a channel that is closed when the connection is closed
	OnClose() <-chan struct{}
	// Close sends a close frame with the specified code and ends the connection
	Close(code events.CloseCode, after time.Duration)
	// Digest returns the message decoder channel utility
	Digest() EventDigest
	// SetWriter defines the connection's writable stream (SSE only)
	SetWriter(w *bufio.Writer)
}

func IsClientSentOp(op events.Opcode) bool {
	switch op {
	case events.OpcodeHeartbeat,
		events.OpcodeIdentify,
		events.OpcodeResume,
		events.OpcodeSubscribe,
		events.OpcodeUnsubscribe,
		events.OpcodeSignal:
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

func NewEventMap(ch chan string) EventMap {
	return EventMap{
		ch:    ch,
		count: utils.PointerOf(int32(0)),
		m:     &sync_map.Map[events.EventType, EventChannel]{},
	}
}

type EventMap struct {
	ch    chan string
	count *int32
	m     *sync_map.Map[events.EventType, EventChannel]
}

// Subscribe sets up a subscription to dispatch events with the specified type
func (e EventMap) Subscribe(gctx global.Context, t events.EventType, cond events.EventCondition, props EventSubscriptionProperties) (EventChannel, uint32, error) {
	id := rand.Uint32()

	ec, exists := e.m.Load(t)
	if !exists {
		ec = EventChannel{
			ID:         []uint32{},
			Conditions: []events.EventCondition{},
			Properties: []EventSubscriptionProperties{props},
		}
	}

	if exists && len(ec.Conditions) == 0 && len(cond) == 0 {
		return ec, id, ErrAlreadySubscribed
	}

	for _, c := range ec.Conditions {
		if c.Match(cond) {
			return ec, id, ErrAlreadySubscribed
		}

	}

	ec.ID = append(ec.ID, id)
	ec.Conditions = append(ec.Conditions, cond)
	ec.Properties = append(ec.Properties, props)

	// Create channel
	e.m.Store(t, ec)

	return ec, id, nil
}

func (e EventMap) Unsubscribe(t events.EventType, cond map[string]string) (uint32, error) {
	if len(cond) == 0 {
		_, exists := e.m.LoadAndDelete(t)
		if !exists {
			return 0, ErrNotSubscribed
		}

		return 0, nil
	}

	ec, exists := e.m.Load(t)
	if !exists {
		return 0, ErrNotSubscribed
	}

	var id uint32

	for i, c := range ec.Conditions {
		if c.Match(cond) {
			id = ec.ID[i]

			utils.SliceRemove(ec.ID, i)
			utils.SliceRemove(ec.Conditions, i)
			break
		}
	}

	return id, nil
}

func (e EventMap) Count() int32 {
	return *e.count
}

func (e EventMap) Get(t events.EventType) (*EventChannel, bool) {
	tWilcard := events.EventType(fmt.Sprintf("%s.*", t.ObjectName()))
	if c, ok := e.m.Load(t); ok {
		return &c, true
	}
	if c, ok := e.m.Load(tWilcard); ok {
		return &c, true
	}

	return nil, false
}

func (e EventMap) DispatchChannel() chan string {
	return e.ch
}

func (e EventMap) Destroy() {
	e.m.Range(func(key events.EventType, value EventChannel) bool {
		e.m.Delete(key)
		return true
	})

	close(e.ch)
}

type EventChannel struct {
	ID         []uint32
	Conditions []events.EventCondition
	Properties []EventSubscriptionProperties
}

type EventSubscriptionProperties struct {
	TTL  time.Time
	Auto bool
}

func (ec EventChannel) Match(cond []events.EventCondition) ([]uint32, bool) {
	if len(ec.Conditions) == 0 { // No condition
		return ec.ID, true
	}

	matches := make(utils.Set[uint32])

	for _, c := range cond {
		for i, e := range ec.Conditions {
			if e.Match(c) {
				matches.Add(ec.ID[i])
			}
		}
	}

	return matches.Values(), len(matches) > 0
}

var (
	ErrAlreadySubscribed = fmt.Errorf("already subscribed")
	ErrNotSubscribed     = fmt.Errorf("not subscribed")
)
