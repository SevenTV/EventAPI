package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/seventv/common/errors"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"

	"github.com/seventv/eventapi/internal/global"
	"github.com/seventv/eventapi/internal/nats"
	"github.com/seventv/eventapi/internal/util"
)

type Server struct {
	upgrader websocket.Upgrader
	router   *chi.Mux

	gctx global.Context

	locked   bool
	shutdown chan struct{}

	activeConns *int32
}

func New(gctx global.Context) (*Server, <-chan struct{}) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		EnableCompression: true,
	}

	srv := Server{
		upgrader: upgrader,
		router:   chi.NewRouter(),

		gctx: gctx,

		shutdown: make(chan struct{}),

		activeConns: new(int32),
	}

	srv.setRoutes()

	srv.HandleSessionMutation(gctx)

	server := http.Server{
		Addr:        srv.gctx.Config().API.Bind,
		IdleTimeout: 30 * time.Second,
		Handler:     srv.router,
		ConnContext: util.SaveConnInContext,
	}

	done := make(chan struct{})
	go func() {
		if err := server.ListenAndServe(); err != nil {
			zap.S().Fatal("failed to start server: ", err)
		}
	}()

	// Watch for file-based shutdown signal
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		zap.S().Fatal("failed to create file watcher: ", err)
	}

	if _, err := os.Create("shutdown"); err != nil {
		zap.S().Fatal("failed to create kill file: ", err)
	}

	if err = watcher.Add("shutdown"); err != nil {
		zap.S().Fatal("failed to add file to watcher: ", err)
	}

	go func() {
		select {
		case <-gctx.Done():
		case <-watcher.Events:
			zap.S().Infof("received api shutdown signal via file system. closing %d connections and shuttering", atomic.LoadInt32(srv.activeConns))
		}

		close(srv.shutdown)
		srv.locked = true

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
		_ = server.Shutdown(context.Background())
		nats.Close()

		watcher.Close()
		close(done)
	}()

	go func() {
		ticker := time.NewTicker(time.Second * 10)

		for {
			select {
			case <-gctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				v := atomic.LoadInt32(srv.activeConns)

				gctx.Inst().ConcurrencyValue = v

				zap.S().Infof("concurrency: %d", v)
			}
		}
	}()

	return &srv, done
}

func (s *Server) Middleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			defer func() {
				l := zap.S().With(
					"status", r.Response.StatusCode,
					"path", r.RequestURI,
					"duration", time.Since(start)/time.Millisecond,
					"ip", r.Header.Get("cf-connecting-ip"),
					"method", r.Method,
					"entrypoint", "api",
				)

				l.Debug("")
			}()
			w.Header().Set("X-Pod-Name", s.gctx.Config().Pod.Name)

			if s.locked {
				writeBytesResponse(http.StatusLocked, []byte("This server is going down for restart!"), w)
				return
			}

			if atomic.LoadInt32(s.activeConns) >= int32(s.gctx.Config().API.ConnectionLimit) {
				writeBytesResponse(http.StatusServiceUnavailable, []byte("This server is full!"), w)
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func (s Server) GetConcurrentConnections() int32 {
	return atomic.LoadInt32(s.activeConns)
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
