package client

import (
	"github.com/seventv/api/data/events"
	"github.com/seventv/eventapi/internal/global"
	"go.uber.org/zap"
)

func HandleDispatch(gctx global.Context, conn Connection, msg events.Message[events.DispatchPayload]) bool {
	// Filter by subscribed event types
	ev, ok := conn.Events().Get(msg.Data.Type)
	if !ok {
		return false // skip if not subscribed to this
	}

	if !ev.Match(msg.Data.Conditions) {
		return false
	}

	// Dedupe
	if msg.Data.Hash != nil {
		h := *msg.Data.Hash

		if !conn.Cache().AddDispatch(h) {
			return false // skip if already dispatched
		}

		msg.Data.Hash = nil
	}

	msg.Data.Conditions = nil

	if err := conn.Write(msg.ToRaw()); err != nil {
		zap.S().Errorw("failed to write dispatch to connection",
			"error", err,
		)

		return false
	}

	return true
}
