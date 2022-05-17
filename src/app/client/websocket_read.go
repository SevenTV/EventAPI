package client

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/Common/utils"
	"github.com/SevenTV/EventAPI/src/global"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
				w.Close(events.CloseCodeInvalidPayload)
				return
			}

			// Verify the opcode
			if !IsClientSentOp(msg.Op) {
				w.Close(events.CloseCodeUnknownOperation)
				return
			}

			switch msg.Op {
			// Handle command - SUBSCRIBE
			case events.OpcodeSubscribe:
				var m events.Message[events.SubscribePayload]
				m, err = events.ConvertMessage[events.SubscribePayload](msg)
				if err != nil {
					goto decodeFailure
				}
				t := m.Data.Type
				path := strings.Split(string(t), ".")
				targets := make([]primitive.ObjectID, len(m.Data.Targets))
				targetCount := 0

				// Parse target IDs
				for _, s := range m.Data.Targets {
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
					w.SendError("Missing event type", nil)
					w.Close(events.CloseCodeInvalidPayload)
					return
				}
				if len(path) < 2 {
					w.SendError("Bad event type path", nil)
					w.Close(events.CloseCodeInvalidPayload)
					return
				}

				// No targets: this requires authentication
				if len(targets) == 0 && w.Actor() == nil {
					w.SendError("Wildcard event target subscription requires authentication", nil)
					w.Close(events.CloseCodeInsufficientPrivilege)
					return
				}

				// Add the event subscription
				_, err = w.Events().Subscribe(gctx, w.ctx, t, targets)
				if err != nil {
					w.Close(events.CloseCodeAlreadySubscribed)
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

	var (
		d   string
		msg events.Message[json.RawMessage]
		err error
	)
	heartbeat := time.NewTicker(time.Duration(w.heartbeatInterval) * time.Millisecond)
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
		case d = <-w.Events().Channel():
			fmt.Println(d)
			err = json.Unmarshal(utils.S2B(d), &msg)
			if err != nil {
				continue
			}
		}
	}
}
