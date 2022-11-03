package monitoring

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/seventv/eventapi/internal/global"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"go.uber.org/zap"
)

func New(gCtx global.Context) <-chan struct{} {
	r := prometheus.NewRegistry()
	gCtx.Inst().Monitoring.Register(r)

	handler := fasthttpadaptor.NewFastHTTPHandler(promhttp.HandlerFor(r, promhttp.HandlerOpts{
		Registry:          r,
		EnableOpenMetrics: true,
	}))

	server := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			defer func() {
				l := zap.S().With(
					"status", ctx.Response.StatusCode(),
					"duration", time.Since(start)/time.Millisecond,
					"entrypoint", "monitoring",
				)
				if err := recover(); err != nil {
					l.Error("panic in handler: ", err)
				} else {
					l.Info("")
				}
			}()
			handler(ctx)
		},
		GetOnly:          true,
		DisableKeepalive: true,
	}

	go func() {
		if err := server.ListenAndServe(gCtx.Config().Monitoring.Bind); err != nil {
			zap.S().Fatal("failed to start monitoring bind: ", err)
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
