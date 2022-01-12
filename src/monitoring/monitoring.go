package monitoring

import (
	"time"

	"github.com/SevenTV/EventAPI/src/global"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

func New(gCtx global.Context) <-chan struct{} {
	r := prometheus.NewRegistry()
	gCtx.Inst().Monitoring.Register(r)

	handler := fasthttpadaptor.NewFastHTTPHandler(promhttp.HandlerFor(r, promhttp.HandlerOpts{
		Registry:          r,
		ErrorLog:          logrus.New(),
		EnableOpenMetrics: true,
	}))

	server := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			defer func() {
				l := logrus.WithFields(logrus.Fields{
					"status":     ctx.Response.StatusCode(),
					"duration":   time.Since(start) / time.Millisecond,
					"entrypoint": "monitoring",
				})
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
			logrus.Fatal("failed to start monitoring bind: ", err)
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
