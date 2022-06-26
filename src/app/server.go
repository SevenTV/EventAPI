package app

import (
	"encoding/json"
	"time"

	"github.com/SevenTV/EventAPI/src/app/client"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/fasthttp/router"
	"github.com/fasthttp/websocket"
	"github.com/seventv/common/errors"
	"github.com/seventv/common/events"
	"github.com/seventv/common/redis"
	"github.com/seventv/common/sync_map"
	"github.com/seventv/common/utils"
	"github.com/valyala/fasthttp"

	"github.com/sirupsen/logrus"
)

type Server struct {
	digest   client.EventDigest
	upgrader websocket.FastHTTPUpgrader
	router   *router.Router
	conns    *sync_map.Map[string, client.Connection]
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
		conns:    &sync_map.Map[string, client.Connection]{},
	}
	srv.HandleConnect(gctx)
	srv.HandleHealth(gctx)
	srv.HandleSessionMutation(gctx)

	server := fasthttp.Server{
		CloseOnShutdown: true,
		Handler: func(ctx *fasthttp.RequestCtx) {
			start := time.Now()
			defer func() {
				l := logrus.WithFields(logrus.Fields{
					"status":     ctx.Response.StatusCode(),
					"path":       utils.B2S(ctx.Request.RequestURI()),
					"duration":   time.Since(start) / time.Millisecond,
					"ip":         utils.B2S(ctx.Request.Header.Peek("cf-connecting-ip")),
					"method":     utils.B2S(ctx.Method()),
					"entrypoint": "api",
				})
				if err := recover(); err != nil {
					l.Error("panic in handler: ", err)
				} else {
					l.Info("")
				}
			}()
			ctx.Response.Header.Set("X-Pod-Name", gctx.Config().Pod.Name)

			r.Handler(ctx)
		},
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := server.ListenAndServe(gctx.Config().API.Bind); err != nil {
			logrus.Fatal("failed to start server: ", err)
		}
	}()

	go func() {
		<-gctx.Done()
		_ = server.Shutdown()
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
