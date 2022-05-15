package v3

import (
	"github.com/SevenTV/Common/structures/v3/events"
	"github.com/SevenTV/EventAPI/src/app/client"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/fasthttp/websocket"
)

func WebSocket(gctx global.Context, conn *websocket.Conn) {
	w := client.NewWebSocket(gctx, conn)

	if err := w.Greet(); err != nil {
		w.Close(events.CloseCodeServerError)
	}

	w.Read(gctx)
}
