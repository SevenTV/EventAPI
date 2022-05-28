package app

import (
	"time"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/Common/utils"
	"github.com/SevenTV/EventAPI/src/app/client"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/fasthttp/router"
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"

	"github.com/sirupsen/logrus"
)

type Server struct {
	digest   client.EventDigest
	upgrader websocket.FastHTTPUpgrader
	router   *router.Router
}

func New(gctx global.Context) (Server, <-chan struct{}) {
	upgrader := websocket.FastHTTPUpgrader{
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
		EnableCompression: true,
	}

	// Connection map (v3 only)
	dig := client.EventDigest{
		Dispatch: client.NewDigest[events.DispatchPayload](gctx, events.OpcodeDispatch, true),
	}

	r := router.New()
	srv := Server{
		upgrader: upgrader,
		digest:   dig,
		router:   r,
	}
	srv.HandleConnect(gctx)
	srv.HandleHealth(gctx)

	server := fasthttp.Server{
		CloseOnShutdown: true,
		Handler: func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			defer func() {
				l := logrus.WithFields(logrus.Fields{
					"status":     ctx.Response.StatusCode(),
					"path":       utils.B2S(ctx.Request.RequestURI()),
					"duration":   time.Since(start) / time.Millisecond,
					"ip":         utils.B2S(ctx.Request.Header.Peek("cf-connecting-ip")),
					"method":     utils.B2S(ctx.Method()),
					"entrypoint": "api",
				})
				if err := recover(); err != nil {
					l.Error("panic in handler: ", err)
				} else {
					l.Info("")
				}
			}()
			ctx.Response.Header.Set("X-Pod-Name", gctx.Config().Pod.Name)

			r.Handler(ctx)
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := server.ListenAndServe(gctx.Config().API.Bind); err != nil {
			logrus.Fatal("failed to start server: ", err)
		}
	}()

	go func() {
		<-gctx.Done()
		_ = server.Shutdown()
	}()

	return srv, done
}
