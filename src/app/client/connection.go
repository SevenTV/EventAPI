package client

import (
	"bufio"
	"context"
	"crypto/rand"
	"fmt"

	"github.com/SevenTV/EventAPI/src/global"
	"github.com/seventv/common/events"
	"github.com/seventv/common/structures/v3"
	"github.com/seventv/common/sync_map"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		ch: ch,
		m:  &sync_map.Map[events.EventType, *EventChannel]{},
	}
}

type EventMap struct {
	ch chan string
	m  *sync_map.Map[events.EventType, *EventChannel]
}

// Subscribe sets up a subscription to dispatch events with the specified type
func (e EventMap) Subscribe(gctx global.Context, t events.EventType, targets []string) (*EventChannel, error) {
	_, exists := e.m.Load(t)
	if exists {
		return nil, ErrAlreadySubscribed
	}

	// Parse targets
	targetIDs := make([]primitive.ObjectID, len(targets))
	targetCount := 0
	for _, s := range targets {
		id, err := primitive.ObjectIDFromHex(s)
		if err == nil {
			targetIDs[targetCount] = id
			targetCount++
		}
	}
	if len(targets) != targetCount {
		targetIDs = targetIDs[:targetCount]
	}

	// Create channel
	ec := &EventChannel{
		targets: targetIDs,
	}
	e.m.Store(t, ec)
	return ec, nil
}

func (e EventMap) Unsubscribe(t events.EventType) error {
	_, exists := e.m.LoadAndDelete(t)
	if !exists {
		return ErrNotSubscribed
	}

	return nil
}

func (e EventMap) Has(t events.EventType) bool {
	tWilcard := events.EventType(fmt.Sprintf("%s.*", t.ObjectName()))
	if _, ok := e.m.Load(t); ok {
		return true
	}
	if _, ok := e.m.Load(tWilcard); ok {
		return true
	}
	return false
}

func (e EventMap) DispatchChannel() chan string {
	return e.ch
}

type EventChannel struct {
	targets []primitive.ObjectID
}

func (ec *EventChannel) Targets() []primitive.ObjectID {
	return ec.targets
}

var (
	ErrAlreadySubscribed = fmt.Errorf("already subscribed")
	ErrNotSubscribed     = fmt.Errorf("not subscribed")
)
