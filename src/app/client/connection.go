package client

import (
	"bufio"
	"fmt"

	"github.com/SevenTV/Common/structures/v3"
	"github.com/SevenTV/Common/structures/v3/events"
	"github.com/SevenTV/Common/sync_map"
	"github.com/SevenTV/EventAPI/src/global"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Connection interface {
	// Greet sends an Hello message to the client
	Greet() error
	// Listen for incoming and outgoing events
	Read(gctx global.Context)
	// Heartbeat lets the client know that the connection is healthy
	Heartbeat() error
	// Dispatch sends a Dispatch event message to the client
	Dispatch(t events.EventType, data []byte) error
	// SendError publishes an error message to the client
	SendError(txt string, fields map[string]any)
	// Close sends a close frame with the specified code and ends the connection
	Close(code events.CloseCode)
	// Actor returns the authenticated user for this connection
	Actor() *structures.User
	// Subscriptions returns an instance of Events
	Events() EventMap
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

func NewEventMap() EventMap {
	return EventMap{
		m: &sync_map.Map[events.EventType, []primitive.ObjectID]{},
	}
}

type EventMap struct {
	m *sync_map.Map[events.EventType, []primitive.ObjectID]
}

func (e EventMap) Subscribe(t events.EventType, targets []primitive.ObjectID) error {
	_, exists := e.m.LoadOrStore(t, targets)
	if exists {
		return ErrAlreadySubscribed
	}
	return nil
}

func (e EventMap) Unsubscribe(t events.EventType) (int, error) {
	a, exists := e.m.LoadAndDelete(t)
	if !exists {
		return 0, ErrNotSubscribed
	}
	return len(a), nil
}

var (
	ErrAlreadySubscribed = fmt.Errorf("already subscribed")
	ErrNotSubscribed     = fmt.Errorf("not subscribed")
)
