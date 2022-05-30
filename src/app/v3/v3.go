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

func WebSocket(gctx global.Context, conn *websocket.Conn, dig client.EventDigest) (client.Connection, error) {
	w, err := client_websocket.NewWebSocket(gctx, conn, dig)
	if err != nil {
		return nil, err
	}

	if err := w.Greet(); err != nil {
		w.Close(events.CloseCodeServerError)
	}

	go w.Read(gctx)
	return w, nil
}

func SSE(gctx global.Context, ctx *fasthttp.RequestCtx, dig client.EventDigest, r *router.Router) (client.Connection, error) {
	es, err := client_eventstream.NewEventStream(gctx, ctx, dig, r)
	if err != nil {
		return nil, err
	}

	client_eventstream.SetupEventStream(ctx, func(w *bufio.Writer) {
		es.SetWriter(w)
		es.Read(gctx)
	})
	return es, nil
}
