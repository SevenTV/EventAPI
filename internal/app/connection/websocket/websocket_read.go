package websocket

import (
	"encoding/json"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/seventv/api/data/events"
	"github.com/seventv/common/utils"
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
}

func (w *WebSocket) Read(gctx global.Context) {
	heartbeat := time.NewTicker(time.Duration(w.heartbeatInterval) * time.Millisecond)

	ttl := time.NewTimer(time.Duration(gctx.Config().API.TTL) * time.Second)

	deferred := false

	defer func() {
		heartbeat.Stop()
		w.Destroy()
		ttl.Stop()
	}()

	go func() {
		<-w.OnReady() // wait for the connection to be ready before accepting input

		// If the connection is closed before it's ready, close it
		if w.ctx.Err() != nil {
			return
		}

		defer func() {
			deferred = true

			// Ignore panics, they're caused by fasthttp
			_ = recover()

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
			err = w.c.ReadJSON(&msg)
			if websocket.IsCloseError(err, ResumableCloseCodes...) {
				w.evbuf = client.NewEventBuffer(w, w.SessionID(), time.Duration(w.heartbeatInterval)*time.Millisecond)
				err := w.evbuf.Start(gctx)
				if err != nil {
					zap.S().Errorw("event buffer start error", "error", err)
				}

				return
			}

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

	var (
		s   string
		err error
	)

	for {
		select {
		case <-w.OnClose():
			return
		case <-gctx.Done():
			w.SendClose(events.CloseCodeRestart, time.Second*5)
			return
		case <-ttl.C:
			w.Write(events.NewMessage(events.OpcodeReconnect, events.ReconnectPayload{
				Reason: "The server requested a reconnect",
			}).ToRaw())
			w.SendClose(events.CloseCodeReconnect, time.Second*5)

			return
		case <-heartbeat.C: // Send a heartbeat
			if !deferred {
				if err := w.SendHeartbeat(); err != nil {
					return
				}
			}
		// Listen for incoming dispatches
		case s = <-w.Events().DispatchChannel():
			var msg events.Message[events.DispatchPayload]

			err = json.Unmarshal(utils.S2B(s), &msg)
			if err != nil {
				zap.S().Errorw("dispatch unmarshal error",
					"error", err,
					"data", s,
				)
				continue
			}

			// Dispatch the event to the client
			w.handler.OnDispatch(gctx, msg)
		}
	}
}
