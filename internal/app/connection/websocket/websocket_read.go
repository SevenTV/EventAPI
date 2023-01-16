package websocket

import (
	"encoding/json"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/seventv/api/data/events"
	client "github.com/seventv/eventapi/internal/app/connection"
	"github.com/seventv/eventapi/internal/global"
)

func (w *WebSocket) Read(gctx global.Context) {
	heartbeat := time.NewTicker(time.Duration(w.heartbeatInterval) * time.Millisecond)
	dispatch := make(chan events.Message[events.DispatchPayload], 128)

	dispatchSub := w.Digest().Dispatch.Subscribe(w.ctx, w.sessionID, dispatch)

	defer func() {
		heartbeat.Stop()
		dispatchSub.Close()
		w.cancel()
		w.evm.Destroy()

		close(w.close)
	}()

	go func() {
		<-w.OnReady() // wait for the connection to be ready before accepting input

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
			if w.c == nil {
				return
			}

			_, data, err = w.c.ReadMessage()
			if websocket.IsUnexpectedCloseError(err) {
				return
			}

			if err != nil {
				w.Close(events.CloseCodeInvalidPayload, 0)
				return
			}

			// Decode the payload
			if err := json.Unmarshal(data, &msg); err != nil {
				w.SendError(err.Error(), nil)
				w.Close(events.CloseCodeInvalidPayload, 0)
				return
			}

			// Verify the opcode
			if !client.IsClientSentOp(msg.Op) {
				w.Close(events.CloseCodeUnknownOperation, 0)
				return
			}

			handler := client.NewHandler(w)
			switch msg.Op {
			// Handle command - SUBSCRIBE
			case events.OpcodeSubscribe:
				if err, _ = handler.Subscribe(gctx, msg); err != nil {
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

	if err := w.Greet(); err != nil {
		close(w.ready)

		return
	}

	close(w.ready) // mark the connection as ready

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-gctx.Done(): // App is shutting down
			w.Close(events.CloseCodeRestart, 0)
			return
		case <-heartbeat.C: // Send a heartbeat
			if err := w.SendHeartbeat(); err != nil {
				return
			}
		// Listen for incoming dispatches
		case msg := <-dispatch:
			_ = w.handler.OnDispatch(gctx, msg)
		}
	}
}
