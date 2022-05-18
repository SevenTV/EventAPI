package client

import (
	"encoding/json"
	"strings"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/EventAPI/src/global"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func NewHandler(conn Connection) Handler {
	return Handler{conn}
}

type Handler struct {
	conn Connection
}

func (h Handler) Subscribe(gctx global.Context, m events.Message[json.RawMessage]) error {
	msg, err := events.ConvertMessage[events.SubscribePayload](m)
	if err != nil {
		return err
	}

	t := msg.Data.Type
	path := strings.Split(string(t), ".")
	targets := make([]primitive.ObjectID, len(msg.Data.Targets))
	targetCount := 0

	// Parse target IDs
	for _, s := range msg.Data.Targets {
		id, err := primitive.ObjectIDFromHex(s)
		if err == nil {
			targets[targetCount] = id
			targetCount++
		}
	}
	if len(targets) != targetCount {
		targets = targets[:targetCount]
	}

	// Empty subscription event type
	if t == "" {
		h.conn.SendError("Missing event type", nil)
		h.conn.Close(events.CloseCodeInvalidPayload)
		return nil
	}
	if len(path) < 2 {
		h.conn.SendError("Bad event type path", nil)
		h.conn.Close(events.CloseCodeInvalidPayload)
		return nil
	}

	// No targets: this requires authentication
	if len(targets) == 0 && h.conn.Actor() == nil {
		h.conn.SendError("Wildcard event target subscription requires authentication", nil)
		h.conn.Close(events.CloseCodeInsufficientPrivilege)
		return nil
	}

	// Add the event subscription
	_, err = h.conn.Events().Subscribe(gctx, h.conn.Context(), t, targets)
	if err != nil {
		if err == ErrAlreadySubscribed {
			h.conn.Close(events.CloseCodeAlreadySubscribed)
			return nil
		}
		return err
	}
	return nil
}
