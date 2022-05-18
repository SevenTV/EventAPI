package websocket

import (
	"encoding/json"
	"io"
	"time"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/EventAPI/src/app/client"
	"github.com/SevenTV/EventAPI/src/global"
)

func (w *WebSocket) Read(gctx global.Context) {
	go func() {
		var (
			data []byte
			msg  events.Message[json.RawMessage]
			err  error
		)
		defer func() {
			w.cancel()
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
			case events.OpcodeUnsubscribe:
				_, err := events.ConvertMessage[events.UnsubscribePayload](msg)
				if err != nil {
					goto decodeFailure
				}

			}
		decodeFailure:
			if err != nil {
				if err == io.EOF {
					w.SendError("Received an empty payload", nil)
					w.Close(events.CloseCodeInvalidPayload)
				} else {
					w.SendError("decode failure", map[string]any{
						"ERROR": err.Error(),
					})
					w.Close(events.CloseCodeInvalidPayload)
				}
				return
			}

		}
	}()

	heartbeat := time.NewTicker(time.Duration(w.heartbeatInterval) * time.Millisecond)
	dispatch := make(chan events.Message[events.DispatchPayload])
	w.Digest().Dispatch.Subscribe(w.ctx, dispatch)

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

			w.c.WriteJSON(msg)
		}
	}
}
