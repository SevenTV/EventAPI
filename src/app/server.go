package app

import (
	"time"

	"github.com/SevenTV/Common/utils"
	v1 "github.com/SevenTV/EventAPI/src/app/v1"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"

	"github.com/sirupsen/logrus"
)

func New(gCtx global.Context) <-chan struct{} {
	upgrader := websocket.FastHTTPUpgrader{
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
		EnableCompression: true,
	}

	server := fasthttp.Server{
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

			ctx.Response.Header.Set("X-Pod-Name", gCtx.Config().Pod.Name)

			switch utils.B2S(ctx.Path()) {
			case "/v1//channel-emotes", "/v1/channel-emotes":
				if utils.B2S(ctx.Request.Header.Peek("upgrade")) == "websocket" {
					if err := upgrader.Upgrade(ctx, func(c *websocket.Conn) {
						v1.ChannelEmotesWS(gCtx, c)
					}); err != nil {
						ctx.SetStatusCode(400)
						ctx.SetBody([]byte(err.Error()))
					}
				} else {
					v1.ChannelEmotesSSE(gCtx, ctx)
				}
			case "/health":
				if err := gCtx.Inst().Redis.Ping(ctx); err != nil {
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

	go func() {
		if err := server.ListenAndServe(gCtx.Config().API.Bind); err != nil {
			logrus.Fatal("failed to start server: ", err)
		}
	}()

	done := make(chan struct{})
	go func() {
		<-gCtx.Done()
		_ = server.Shutdown()
		close(done)
	}()

	return done
}
