package v3

import (
	"bufio"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/EventAPI/src/app/client"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

func WebSocket(gctx global.Context, conn *websocket.Conn) {
	w, err := client.NewWebSocket(gctx, conn)
	if err != nil {
		return
	}

	if err := w.Greet(); err != nil {
		w.Close(events.CloseCodeServerError)
	}

	w.Read(gctx)
}

func SSE(gctx global.Context, ctx *fasthttp.RequestCtx) {
	es, err := client.NewSSE(gctx, ctx)
	if err != nil {
		return
	}

	client.SetupEventStream(ctx, func(w *bufio.Writer) {
		es.SetWriter(w)
		es.Read(gctx)
	})
}
