package health

import (
	"context"
	"time"

	"github.com/seventv/common/utils"
	"github.com/seventv/eventapi/internal/app"
	"github.com/seventv/eventapi/internal/global"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func New(gctx global.Context, srv *app.Server) <-chan struct{} {
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

			ctx.Response.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
			ctx.SetStatusCode(200)
			redisCtx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()

			if err := gctx.Inst().Redis.Ping(redisCtx); err != nil {
				zap.S().Error("redis down: ", err)

				ctx.SetBodyString("Redis Down")
				ctx.SetStatusCode(503)
			}

			// check if path is /concurrency
			if !ctx.IsGet() || utils.B2S(ctx.URI().Path()) == "/concurrency" && srv != nil && srv.GetConcurrentConnections() >= (gctx.Config().API.ConnectionLimit-1) {
				zap.S().Warnw("connection limit reached")

				ctx.SetBodyString("Maximum Concurrency")
				ctx.SetStatusCode(410)
			}
		},
		GetOnly:          true,
		DisableKeepalive: true,
	}

	go func() {
		if err := server.ListenAndServe(gctx.Config().Health.Bind); err != nil {
			zap.S().Fatal("failed to start health bind: ", err)
		}
	}()

	done := make(chan struct{})
	go func() {
		<-gctx.Done()
		_ = server.Shutdown()
		close(done)
	}()
	return done
}
