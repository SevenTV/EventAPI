package websocket

import (
	"encoding/json"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/seventv/api/data/events"
	client "github.com/seventv/eventapi/internal/app/connection"
	"github.com/seventv/eventapi/internal/global"
	"go.uber.org/zap"
)

var ResumableCloseCodes = []int{
	websocket.CloseNormalClosure,
	websocket.CloseGoingAway,
	websocket.CloseAbnormalClosure,
	int(events.CloseCodeTimeout),
	int(events.OpcodeReconnect),
	int(events.CloseCodeRestart),
}

func (w *WebSocket) Read(gctx global.Context) {
	heartbeat := time.NewTicker(time.Duration(w.heartbeatInterval) * time.Millisecond)
	dispatch := w.Digest().Dispatch.Subscribe(w.ctx, w.sessionID, 1024)

	deferred := false

	defer func() {
		heartbeat.Stop()
		w.Destroy()
	}()

	go func() {
		<-w.OnReady() // wait for the connection to be ready before accepting input

		// If the connection is closed before it's ready, close it
		if w.ctx.Err() != nil {
			return
		}

		defer func() {
			deferred = true

			if r := recover(); r != nil {
				zap.S().Errorw("websocket read panic", "error", r)
			}

			buf := w.Buffer()
			if buf != nil {
				<-buf.Context().Done()
			}

			w.Destroy()
		}()

		var msg events.Message[json.RawMessage]
		var err error

		// Listen for incoming messages sent by the client
		for {
			if w.c == nil {
				break
			}

			err = w.c.ReadJSON(&msg)

			if websocket.IsUnexpectedCloseError(err) {
				return
			}

			if err != nil {
				w.SendError(err.Error(), nil)
				w.SendClose(events.CloseCodeInvalidPayload, 0)
				return
			}

			// Verify the opcode
			if !client.IsClientSentOp(msg.Op) {
				w.SendClose(events.CloseCodeUnknownOperation, 0)
				return
			}

			handler := client.NewHandler(w)
			switch msg.Op {
			// Handle command - RESUME
			case events.OpcodeResume:
				if err = handler.OnResume(gctx, msg); err != nil {
					return
				}
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
			// Handle command - BRIDGE
			case events.OpcodeBridge:
				if err = handler.OnBridge(gctx, msg); err != nil {
					return
				}
			}
		}
	}()

	if err := w.Greet(); err != nil {
		return
	}

	w.SetReady() // mark the connection as ready

	for {
		select {
		case <-w.OnClose():
			return
		case <-gctx.Done():
			w.SendClose(events.CloseCodeRestart, time.Second*5)
			return
		case <-heartbeat.C: // Send a heartbeat
			if !deferred {
				if err := w.SendHeartbeat(); err != nil {
					return
				}
			}
		// Listen for incoming dispatches
		case msg := <-dispatch:
			// Dispatch the event to the client
			_ = w.handler.OnDispatch(gctx, msg)
		}
	}
}
