package v3

import (
	"bufio"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/EventAPI/src/app/client"
	client_eventstream "github.com/SevenTV/EventAPI/src/app/client/eventstream"
	client_websocket "github.com/SevenTV/EventAPI/src/app/client/websocket"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/fasthttp/router"
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

func WebSocket(gctx global.Context, conn *websocket.Conn, dig client.EventDigest) client.Connection {
	w, err := client_websocket.NewWebSocket(gctx, conn, dig)
	if err != nil {
		return nil
	}

	if err := w.Greet(); err != nil {
		w.Close(events.CloseCodeServerError)
	}

	w.Read(gctx)
	return w
}

func SSE(gctx global.Context, ctx *fasthttp.RequestCtx, dig client.EventDigest, r *router.Router) client.Connection {
	es, err := client_eventstream.NewSSE(gctx, ctx, dig, r)
	if err != nil {
		return nil
	}

	client_eventstream.SetupEventStream(ctx, func(w *bufio.Writer) {
		es.SetWriter(w)
		es.Read(gctx)
	})
	return es
}
