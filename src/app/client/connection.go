package client

import (
	"github.com/SevenTV/Common/structures/v3/events"
	"github.com/SevenTV/EventAPI/src/global"
)

type Connection interface {
	// Greet sends an Hello message to the client
	Greet() error
	// Listen for incoming and outgoing events
	Read(gctx global.Context)
	// Dispatch sends a standard message
	Dispatch(t events.MessageType, data []byte) error
	// Close the connection with the specified code
	Close(code events.CloseCode)
}

func IsClientSentOp(op events.Opcode) bool {
	switch op {
	case events.OpcodeHeartbeat,
		events.OpcodeIdentify,
		events.OpcodeResume,
		events.OpcodeSubscribe,
		events.OpcodeSignalPresence:
		return true
	default:
		return false
	}
}
