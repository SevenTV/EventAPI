package app

import (
	"github.com/SevenTV/Common/utils"
	v1 "github.com/SevenTV/EventAPI/src/app/v1"
	v3 "github.com/SevenTV/EventAPI/src/app/v3"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

func (s Server) HandleConnect(gctx global.Context) {
	s.router.GET("/v3", func(ctx *fasthttp.RequestCtx) {
		if utils.B2S(ctx.Request.Header.Peek("upgrade")) == "websocket" {
			if err := s.upgrader.Upgrade(ctx, func(c *websocket.Conn) {
				_ = v3.WebSocket(gctx, c, s.digest)
			}); err != nil {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBody(utils.S2B(err.Error()))
			}
		} else {
			_ = v3.SSE(gctx, ctx, s.digest, s.router)
		}
	})

	s.router.GET("/v1/channel-emotes", func(ctx *fasthttp.RequestCtx) {
		if utils.B2S(ctx.Request.Header.Peek("upgrade")) == "websocket" {
			if err := s.upgrader.Upgrade(ctx, func(c *websocket.Conn) {
				v1.ChannelEmotesWS(gctx, c)
			}); err != nil {
				ctx.SetStatusCode(400)
				ctx.SetBody([]byte(err.Error()))
			}
		} else {
			v1.ChannelEmotesSSE(gctx, ctx)
		}

	})
}
