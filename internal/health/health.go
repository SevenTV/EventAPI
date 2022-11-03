package health

import (
	"context"
	"time"

	"github.com/seventv/eventapi/internal/global"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func New(gCtx global.Context) <-chan struct{} {
	server := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			defer func() {
				l := zap.S().With(
					"status", ctx.Response.StatusCode(),
					"duration", time.Since(start)/time.Millisecond,
					"entrypoint", "health",
				)
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
				zap.S().Error("redis down: ", err)
				ctx.SetStatusCode(503)
			}
		},
		GetOnly:          true,
		DisableKeepalive: true,
	}

	go func() {
		if err := server.ListenAndServe(gCtx.Config().Health.Bind); err != nil {
			zap.S().Fatal("failed to start health bind: ", err)
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
