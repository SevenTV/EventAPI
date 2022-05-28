package app

import (
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/valyala/fasthttp"
)

func (s Server) HandleHealth(gctx global.Context) {
	s.router.GET("/health", func(ctx *fasthttp.RequestCtx) {
		if err := gctx.Inst().Redis.Ping(ctx); err != nil {
			ctx.SetBodyString("redis down")
			ctx.SetStatusCode(fasthttp.StatusServiceUnavailable)
		} else {
			ctx.SetBodyString("OK")
			ctx.SetStatusCode(fasthttp.StatusOK)
		}
	})
}
