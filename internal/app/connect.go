package app

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/seventv/common/utils"
	client "github.com/seventv/eventapi/internal/app/connection"
	v1 "github.com/seventv/eventapi/internal/app/v1"
	v3 "github.com/seventv/eventapi/internal/app/v3"
	"github.com/seventv/eventapi/internal/global"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func (s Server) HandleConnect(gctx global.Context) {
	v3Fn := func(ctx *fasthttp.RequestCtx) {
		var (
			con client.Connection
			err error
		)

		connected := make(chan bool, 1)

		if utils.B2S(ctx.Request.Header.Peek("upgrade")) == "websocket" {
			if err := s.upgrader.Upgrade(ctx, func(c *websocket.Conn) { // New WebSocket connection
				con, err = v3.WebSocket(gctx, c, s.digest)
				fmt.Println("hi?", con)
				if err != nil {
					ctx.SetStatusCode(fasthttp.StatusBadRequest)
					ctx.SetBody(utils.S2B(err.Error()))
					return
				}

				connected <- true
				<-con.Context().Done()
			}); err != nil {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBody(utils.S2B(err.Error()))
				return
			}
		} else { // New EventStream connection
			con, err = v3.SSE(gctx, ctx, s.digest, s.router)
			if err != nil {
				ctx.SetStatusCode(fasthttp.StatusBadRequest)
				ctx.SetBody(utils.S2B(err.Error()))
				return
			}

			connected <- true
		}

		go func() {
			ok := <-connected

			close(connected)

			if !ok {
				return
			}

			s.TrackConnection(gctx, ctx, con)
		}()
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

func (s Server) TrackConnection(gctx global.Context, ctx *fasthttp.RequestCtx, con client.Connection) {
	if con == nil {
		return
	}

	zap.S().Debugw("new connection",
		"client_addr", ctx.RemoteAddr().String(),
		"connection_count", atomic.LoadInt32(s.activeConns),
	)

	<-con.OnReady() // wait for connection to be ready

	start := time.Now()

	// Increment counters
	atomic.AddInt32(s.activeConns, 1)

	gctx.Inst().Monitoring.EventV3().CurrentConnections.Inc()
	gctx.Inst().Monitoring.EventV3().TotalConnections.Observe(1)

	<-con.OnClose() // wait for connection to end

	// Decrement counters
	atomic.AddInt32(s.activeConns, -1)

	gctx.Inst().Monitoring.EventV3().CurrentConnections.Dec()
	gctx.Inst().Monitoring.EventV3().TotalConnections.Observe(float64(time.Since(start)/time.Millisecond) / 1000)

	zap.S().Debugw("connection ended",
		"client_addr", ctx.RemoteAddr().String(),
		"connection_count", atomic.LoadInt32(s.activeConns),
	)
}
