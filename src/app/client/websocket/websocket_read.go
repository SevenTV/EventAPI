package websocket

import (
	"encoding/json"
	"time"

	"github.com/SevenTV/EventAPI/src/app/client"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/seventv/common/events"
	"go.uber.org/zap"
)

func (w *WebSocket) Read(gctx global.Context) {
	heartbeat := time.NewTicker(time.Duration(w.heartbeatInterval) * time.Millisecond)
	dispatch := make(chan events.Message[events.DispatchPayload])
	go func() {
		w.Digest().Dispatch.Subscribe(w.ctx, w.sessionID, dispatch)
	}()

	go func() {
		var (
			data []byte
			msg  events.Message[json.RawMessage]
			err  error
		)
		defer func() {
			w.cancel()
			close(dispatch)
		}()

		// Listen for incoming messages sent by the client
		for {
			_, data, err = w.c.ReadMessage()
			if err != nil {
				w.Close(events.CloseCodeInvalidPayload)
				return
			}

			// Decode the payload
			if err := json.Unmarshal(data, &msg); err != nil {
				w.SendError(err.Error(), nil)
				w.Close(events.CloseCodeInvalidPayload)
				return
			}

			// Verify the opcode
			if !client.IsClientSentOp(msg.Op) {
				w.Close(events.CloseCodeUnknownOperation)
				return
			}

			handler := client.NewHandler(w)
			switch msg.Op {
			// Handle command - SUBSCRIBE
			case events.OpcodeSubscribe:
				if err = handler.Subscribe(gctx, msg); err != nil {
					return
				}
			// Handle command - UNSUBSCRIBE
			case events.OpcodeUnsubscribe:
				if err = handler.Unsubscribe(gctx, msg); err != nil {
					return
				}
			}

		}
	}()

	for {
		select {
		case <-w.ctx.Done(): // App is shutting down
			w.Close(events.CloseCodeRestart)
			return
		case <-heartbeat.C: // Send a heartbeat
			if err := w.Heartbeat(); err != nil {
				w.Close(events.CloseCodeTimeout)
				return
			}
		// Listen for incoming dispatches
		case msg := <-dispatch:
			// Filter by the connection's subscribed events
			if !w.Events().Has(msg.Data.Type) {
				continue // skip if not subscribed to this
			}

			if err := w.c.WriteJSON(msg); err != nil {
				zap.S().Errorw("failed to write dispatch to connection",
					"error", err,
				)
				continue
			}
		}
	}
}
