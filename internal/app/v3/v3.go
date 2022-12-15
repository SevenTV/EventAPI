package v3

import (
	"bufio"
	"net/url"
	"regexp"
	"strings"

	"github.com/fasthttp/router"
	"github.com/fasthttp/websocket"
	"github.com/seventv/api/data/events"
	client "github.com/seventv/eventapi/internal/app/connection"
	client_eventstream "github.com/seventv/eventapi/internal/app/connection/eventstream"
	client_websocket "github.com/seventv/eventapi/internal/app/connection/websocket"
	"github.com/seventv/eventapi/internal/global"
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

var (
	SSE_SUBSCRIPTION_ITEM       = regexp.MustCompile(`(?P<EVT>^\w+\.[a-zA-Z0-9*]+)(\<(?P<CND>.+)\>)?`)
	SSE_SUBSCRIPTION_ITEM_I_EVT = SSE_SUBSCRIPTION_ITEM.SubexpIndex("EVT")
	SSE_SUBSCRIPTION_ITEM_I_CND = SSE_SUBSCRIPTION_ITEM.SubexpIndex("CND")
)

func SSE(gctx global.Context, ctx *fasthttp.RequestCtx, dig client.EventDigest, r *router.Router) (client.Connection, error) {
	es, err := client_eventstream.NewEventStream(gctx, ctx, dig, r)
	if err != nil {
		return nil, err
	}

	// Parse subscriptions
	sub := ctx.UserValue("sub")
	switch s := sub.(type) {
	case string:
		s, _ = url.QueryUnescape(s)
		if s == "" || !strings.HasPrefix(s, "@") {
			break
		}

		subStrs := strings.Split(s[1:], ",")

		for _, subStr := range subStrs {
			matches := SSE_SUBSCRIPTION_ITEM.FindStringSubmatch(subStr)
			if len(matches) == 0 {
				continue
			}

			evt := matches[SSE_SUBSCRIPTION_ITEM_I_EVT]
			cnd := matches[SSE_SUBSCRIPTION_ITEM_I_CND]

			conds := strings.Split(cnd, ";")
			cm := make(map[string]string)

			for _, cond := range conds {
				kv := strings.Split(cond, "=")
				if len(kv) != 2 {
					continue
				}

				cm[kv[0]] = kv[1]
			}

			es.Events().Subscribe(gctx, events.EventType(evt), cm)

		}
	}

	client_eventstream.SetupEventStream(ctx, func(w *bufio.Writer) {
		es.SetWriter(w)
		es.Read(gctx)
	})
	return es, nil
}
