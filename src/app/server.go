package app

import (
	"time"

	"github.com/SevenTV/Common/utils"
	v1 "github.com/SevenTV/EventAPI/src/app/v1"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/valyala/fasthttp"

	"github.com/sirupsen/logrus"
)

func New(gCtx global.Context) <-chan struct{} {
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
				v1.ChannelEmotes(gCtx, ctx)
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
