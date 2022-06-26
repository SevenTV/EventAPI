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
	s.router.GET("/v3", func(ctx *fasthttp.RequestCtx) {
		if utils.B2S(ctx.Request.Header.Peek("upgrade")) == "websocket" {
			if err := s.upgrader.Upgrade(ctx, func(c *websocket.Conn) { // New WebSocket connection
				con, err := v3.WebSocket(gctx, c, s.digest)
				if err != nil {
					ctx.SetStatusCode(fasthttp.StatusBadRequest)
					ctx.SetBody(utils.S2B(err.Error()))
					return
				}

				sid := con.SessionID()
				s.conns.Store(sid, con)
				<-con.Context().Done()
				s.conns.Delete(sid)
			}); err != nil {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBody(utils.S2B(err.Error()))
				return
			}
		} else { // New EventStream connection
			con, err := v3.SSE(gctx, ctx, s.digest, s.router)
			if err != nil {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBody(utils.S2B(err.Error()))
				return
			}
			sid := con.SessionID()
			s.conns.Store(sid, con)
			go func() {
				<-con.Context().Done()
				s.conns.Delete(sid)
			}()
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
