package v3

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/seventv/api/data/events"

	client "github.com/seventv/eventapi/internal/app/connection"
	"github.com/seventv/eventapi/internal/global"
)

func WebSocket(gctx global.Context, con client.Connection) error {
	go con.Read(gctx)

	return nil
}

var (
	SSE_SUBSCRIPTION_ITEM       = regexp.MustCompile(`(?P<EVT>^\w+\.[a-zA-Z0-9*]+)(\<(?P<CND>.+)\>)?`)
	SSE_SUBSCRIPTION_ITEM_I_EVT = SSE_SUBSCRIPTION_ITEM.SubexpIndex("EVT")
	SSE_SUBSCRIPTION_ITEM_I_CND = SSE_SUBSCRIPTION_ITEM.SubexpIndex("CND")
)

func SSE(gctx global.Context, conn client.Connection, w http.ResponseWriter, r *http.Request) error {
	f, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("EventStream Not Supported")
	}

	conn.SetWriter(bufio.NewWriter(w), f)

	go func() {
		<-conn.OnReady() // wait for the connection to be ready
		if conn.Context().Err() != nil {
			return
		}

		// Parse subscriptions
		sub := chi.URLParam(r, "sub")

		s, _ := url.QueryUnescape(sub)
		if s == "" || !strings.HasPrefix(s, "@") {
			return
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

			if err, ok := conn.Handler().Subscribe(gctx, events.NewMessage(events.OpcodeSubscribe, events.SubscribePayload{
				Type:      events.EventType(evt),
				Condition: cm,
			}).ToRaw()); err != nil || !ok {
				return
			}
		}
	}()

	conn.Read(gctx)

	return nil
}
