package health

import (
	"context"
	"time"

	"github.com/SevenTV/EventAPI/src/global"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

func New(gCtx global.Context) <-chan struct{} {
	server := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			defer func() {
				l := logrus.WithFields(logrus.Fields{
					"status":     ctx.Response.StatusCode(),
					"duration":   time.Since(start) / time.Millisecond,
					"entrypoint": "health",
				})
				if err := recover(); err != nil {
					l.Error("panic in handler: ", err)
				} else {
					l.Info("")
				}
			}()

			ctx.SetStatusCode(200)
			redisCtx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()

			if err := gCtx.Inst().Redis.Ping(redisCtx); err != nil {
				logrus.Error("redis down: ", err)
				ctx.SetStatusCode(503)
			}
		},
		GetOnly:          true,
		DisableKeepalive: true,
	}

	go func() {
		if err := server.ListenAndServe(gCtx.Config().Health.Bind); err != nil {
			logrus.Fatal("failed to start health bind: ", err)
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
