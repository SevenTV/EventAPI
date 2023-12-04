package eventstream

import (
	"encoding/json"
	"time"

	"github.com/seventv/api/data/events"
	"go.uber.org/zap"

	"github.com/seventv/eventapi/internal/global"
)

func (es *EventStream) Read(gctx global.Context) {
	heartbeat := time.NewTicker(time.Duration(es.heartbeatInterval) * time.Millisecond)

	liveness := time.NewTicker(time.Second * 1)

	defer func() {
		heartbeat.Stop()
		es.Destroy()
		liveness.Stop()
	}()

	if err := es.Greet(gctx); err != nil {
		return
	}

	es.SetReady()

	for {
		select {
		case <-es.r.Context().Done():
			return
		case <-es.OnClose():
			return
		case <-gctx.Done():
			es.SendClose(events.CloseCodeRestart, time.Second*5)
			return
		case <-heartbeat.C:
			if err := es.SendHeartbeat(); err != nil {
				return
			}
		case <-liveness.C: // Connection liveness check
		case s := <-es.evm.DispatchChannel():
			if s == nil { // channel closed
				return
			}

			var msg events.Message[events.DispatchPayload]

			err := json.Unmarshal(s, &msg)
			if err != nil {
				zap.S().Errorw("dispatch unmarshal error", "error", err)
				continue
			}

			// Dispatch the event to the client
			es.handler.OnDispatch(gctx, msg)
		}
	}
}
