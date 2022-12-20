package app

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/fasthttp/router"
	"github.com/fasthttp/websocket"
	"github.com/seventv/api/data/events"
	"github.com/seventv/common/errors"
	"github.com/seventv/common/redis"
	"github.com/seventv/common/utils"
	client "github.com/seventv/eventapi/internal/app/connection"
	"github.com/seventv/eventapi/internal/global"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type Server struct {
	digest   client.EventDigest
	upgrader websocket.FastHTTPUpgrader
	router   *router.Router

	activeConns *int32
}

func New(gctx global.Context) (Server, <-chan struct{}) {
	upgrader := websocket.FastHTTPUpgrader{
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
		EnableCompression: true,
	}

	// Connection map (v3 only)
	dig := client.EventDigest{
		Dispatch: client.NewDigest[events.DispatchPayload](gctx, redis.Key(events.OpcodeDispatch.PublishKey())),
		Ack:      client.NewDigest[events.AckPayload](gctx, redis.Key(events.OpcodeAck.PublishKey())),
	}

	r := router.New()
	srv := Server{
		upgrader: upgrader,
		digest:   dig,
		router:   r,

		activeConns: new(int32),
	}

	srv.HandleConnect(gctx)
	srv.HandleHealth(gctx)
	srv.HandleSessionMutation(gctx)

	server := fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			defer func() {
				l := zap.S().With(
					"status", ctx.Response.StatusCode(),
					"path", utils.B2S(ctx.Request.RequestURI()),
					"duration", time.Since(start)/time.Millisecond,
					"ip", utils.B2S(ctx.Request.Header.Peek("cf-connecting-ip")),
					"method", utils.B2S(ctx.Method()),
					"entrypoint", "api",
				)
				if err := recover(); err != nil {
					l.Error("panic in handler: ", err)
				} else {
					l.Debug("")
				}
			}()
			ctx.Response.Header.Set("X-Pod-Name", gctx.Config().Pod.Name)

			r.Handler(ctx)
		},
	}

	done := make(chan struct{})
	go func() {
		if err := server.ListenAndServe(gctx.Config().API.Bind); err != nil {
			zap.S().Fatal("failed to start server: ", err)
		}
	}()

	go func() {
		<-gctx.Done()

		timeout := time.After(time.Second * 30)
		ticker := time.NewTicker(time.Millisecond * 100)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				goto shutdown
			case <-ticker.C:
				if atomic.LoadInt32(srv.activeConns) == 0 {
					goto shutdown
				}
			}
		}

	shutdown:
		<-time.After(time.Second * 5)

		_ = server.Shutdown()

		close(done)
	}()

	return srv, done
}

type ErrorResponse struct {
	Status     string         `json:"status"`
	StatusCode int            `json:"status_code"`
	Error      string         `json:"error"`
	ErrorCode  int            `json:"error_code"`
	Details    map[string]any `json:"details"`
}

func DoErrorResponse(ctx *fasthttp.RequestCtx, e errors.APIError) {
	b, err := json.Marshal(&ErrorResponse{
		Status:     fasthttp.StatusMessage(e.ExpectedHTTPStatus()),
		StatusCode: e.ExpectedHTTPStatus(),
		Error:      e.Message(),
		ErrorCode:  e.Code(),
		Details:    e.GetFields(),
	})
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	ctx.SetContentType("application/json")
	ctx.SetStatusCode(e.ExpectedHTTPStatus())
	ctx.SetBody(b)
}
