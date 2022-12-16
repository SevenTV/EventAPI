package app

import (
	"github.com/fasthttp/websocket"
	"github.com/seventv/common/utils"
	v1 "github.com/seventv/eventapi/internal/app/v1"
	v3 "github.com/seventv/eventapi/internal/app/v3"
	"github.com/seventv/eventapi/internal/global"
	"github.com/valyala/fasthttp"
)

func (s Server) HandleConnect(gctx global.Context) {
	v3Fn := func(ctx *fasthttp.RequestCtx) {
		if utils.B2S(ctx.Request.Header.Peek("upgrade")) == "websocket" {
			if err := s.upgrader.Upgrade(ctx, func(c *websocket.Conn) { // New WebSocket connection
				con, err := v3.WebSocket(gctx, c, s.digest)
				if err != nil {
					ctx.SetStatusCode(fasthttp.StatusBadRequest)
					ctx.SetBody(utils.S2B(err.Error()))
					return
				}

				<-con.Context().Done()
			}); err != nil {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBody(utils.S2B(err.Error()))
				return
			}
		} else { // New EventStream connection
			_, err := v3.SSE(gctx, ctx, s.digest, s.router)
			if err != nil {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBody(utils.S2B(err.Error()))
				return
			}
		}
	}

	s.router.GET("/v3{sub?:\\@(.*)}", v3Fn)
	s.router.GET("/v3", v3Fn)

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
