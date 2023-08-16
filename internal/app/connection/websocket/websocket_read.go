package websocket

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/seventv/api/data/events"
	"github.com/seventv/common/utils"
	"go.uber.org/zap"

	client "github.com/seventv/eventapi/internal/app/connection"
	"github.com/seventv/eventapi/internal/global"
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

	ttl := time.NewTimer(time.Duration(gctx.Config().API.TTL) * time.Minute)

	deferred := false

	defer func() {
		heartbeat.Stop()
		w.Destroy(gctx)
		ttl.Stop()
	}()

	go func() {
		<-w.OnReady() // wait for the connection to be ready before accepting input

		// If the connection is closed before it's ready, close it
		if w.ctx.Err() != nil {
			return
		}

		// Subscribe to whispers
		if _, _, err := w.evm.Subscribe(gctx, w.ctx, events.EventTypeWhisper, events.EventCondition{
			"session_id": w.SessionID(),
		}, client.EventSubscriptionProperties{
			Auto: true,
		}); err != nil {
			zap.S().Errorw("whisper subscription error", "error", err, "session_id", w.SessionID())
		}

		defer func() {
			deferred = true

			buf := w.Buffer()
			if buf != nil {
				<-buf.Context().Done()
			}

			w.Destroy(gctx)
		}()

		throttle := utils.NewThrottle(time.Millisecond * 100)

		var msg events.Message[json.RawMessage]
		var err error

		// Listen for incoming messages sent by the client
		for {
			err = w.c.ReadJSON(&msg)
			if websocket.IsCloseError(err, ResumableCloseCodes...) {
				// w.evbuf = client.NewEventBuffer(w, w.SessionID(), time.Duration(w.heartbeatInterval)*time.Millisecond)
				// err := w.evbuf.Start(gctx)
				// if err != nil {
				// 	zap.S().Errorw("event buffer start error", "error", err)
				// }

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
				throttle.Do(func() {
					if err = handler.OnBridge(gctx, msg); err != nil {
						return
					}
				})
			}
		}
	}()

	if err := w.Greet(gctx); err != nil {
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
		case <-ttl.C:
			_ = w.Write(events.NewMessage(events.OpcodeReconnect, events.ReconnectPayload{
				Reason: "The server requested a reconnect",
			}).ToRaw())
			w.SendClose(events.CloseCodeReconnect, 0)

			return
		case <-heartbeat.C: // Send a heartbeat
			if !deferred {
				if err := w.SendHeartbeat(); err != nil {
					return
				}
			}
		// Listen for incoming dispatches
		case s := <-w.Events().DispatchChannel():
			if s == nil { // The channel is closed - stop listening
				return
			}

			var msg events.Message[events.DispatchPayload]

			err := json.Unmarshal([]byte(*s), &msg)
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
