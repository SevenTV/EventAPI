package client

import (
	"context"
	"encoding/json"

	"github.com/SevenTV/Common/structures/v3/events"
	"github.com/SevenTV/EventAPI/src/global"
)

func (w *WebSocket) Read(gctx global.Context) {
	lctx, cancel := context.WithCancel(gctx)

	go func() {

		var (
			data []byte
			msg  events.Message[json.RawMessage]
			err  error
		)
		defer func() {
			cancel()
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
		}
	}()

	for {
		select {
		case <-lctx.Done():
			w.Close(events.CloseCodeRestart)
			return
		}
	}
}
