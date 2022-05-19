package app

import (
	"time"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/Common/utils"
	"github.com/SevenTV/EventAPI/src/app/client"
	v1 "github.com/SevenTV/EventAPI/src/app/v1"
	v3 "github.com/SevenTV/EventAPI/src/app/v3"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"

	"github.com/sirupsen/logrus"
)

func New(gctx global.Context) <-chan struct{} {
	upgrader := websocket.FastHTTPUpgrader{
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
		EnableCompression: true,
	}

	dig := client.EventDigest{
		Dispatch: client.NewDigest[events.DispatchPayload](gctx, events.OpcodeDispatch),
	}

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

			switch utils.B2S(ctx.Path()) {
			case "/v3":
				if utils.B2S(ctx.Request.Header.Peek("upgrade")) == "websocket" {
					if err := upgrader.Upgrade(ctx, func(c *websocket.Conn) {
						v3.WebSocket(gctx, c, dig)
					}); err != nil {
						ctx.SetStatusCode(400)
						ctx.SetBody(utils.S2B(err.Error()))
					}
				} else {
					v3.SSE(gctx, ctx, dig)
				}
			case "/v1/channel-emotes":
				if utils.B2S(ctx.Request.Header.Peek("upgrade")) == "websocket" {
					if err := upgrader.Upgrade(ctx, func(c *websocket.Conn) {
						v1.ChannelEmotesWS(gctx, c)
					}); err != nil {
						ctx.SetStatusCode(400)
						ctx.SetBody([]byte(err.Error()))
					}
				} else {
					v1.ChannelEmotesSSE(gctx, ctx)
				}
			case "/health":
				if err := gctx.Inst().Redis.Ping(ctx); err != nil {
					ctx.SetBodyString("redis down")
					ctx.SetStatusCode(500)
				} else {
					ctx.SetBodyString("OK")
					ctx.SetStatusCode(200)
				}
			default:
				ctx.SetStatusCode(404)
			}
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

	return done
}
