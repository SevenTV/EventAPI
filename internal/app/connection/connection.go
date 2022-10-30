package client

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"
	"sync/atomic"

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
	// Heartbeat lets the client know that the connection is healthy
	Heartbeat() error
	// SendError publishes an error message to the client
	SendError(txt string, fields map[string]any)
	// Close sends a close frame with the specified code and ends the connection
	Close(code events.CloseCode)
	// Actor returns the authenticated user for this connection
	Actor() *structures.User
	// Subscriptions returns an instance of Events
	Events() EventMap
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
func (e EventMap) Subscribe(gctx global.Context, t events.EventType, cond map[string]string) (EventChannel, error) {
	ec, exists := e.m.Load(t)
	if !exists {
		ec = make(EventChannel)
	}

	for k, v := range cond {
		if _, ok := ec[k]; !ok {
			ec[k] = make(utils.Set[string])
		}

		if ec[k].Has(v) {
			return ec, ErrAlreadySubscribed
		}

		ec[k].Add(v)

		atomic.AddInt32(e.count, 1)
	}

	// Create channel
	e.m.Store(t, ec)

	return ec, nil
}

func (e EventMap) Unsubscribe(t events.EventType, cond map[string]string) error {
	if len(cond) == 0 {
		_, exists := e.m.LoadAndDelete(t)
		if !exists {
			return ErrNotSubscribed
		}

		return nil
	}

	ec, exists := e.m.Load(t)
	if !exists {
		return ErrNotSubscribed
	}

	x := 0
	for k, v := range cond {
		if !ec[k].Has(v) {
			continue
		}

		ec[k].Delete(v)

		atomic.AddInt32(e.count, -1)

		x++
	}

	if x == 0 {
		return ErrNotSubscribed
	}

	return nil
}

func (e EventMap) Count() int32 {
	return *e.count
}

func (e EventMap) Get(t events.EventType) (EventChannel, bool) {
	tWilcard := events.EventType(fmt.Sprintf("%s.*", t.ObjectName()))
	if c, ok := e.m.Load(t); ok {
		return c, true
	}
	if c, ok := e.m.Load(tWilcard); ok {
		return c, true
	}
	return nil, false
}

func (e EventMap) DispatchChannel() chan string {
	return e.ch
}

type EventChannel map[string]utils.Set[string]

func (ec EventChannel) Match(cond map[string]string) bool {
	for k, v := range cond {
		s, ok := ec[k]
		if !ok {
			return false
		}

		if !s.Has(v) {
			return false
		}
	}

	return true
}

var (
	ErrAlreadySubscribed = fmt.Errorf("already subscribed")
	ErrNotSubscribed     = fmt.Errorf("not subscribed")
)
