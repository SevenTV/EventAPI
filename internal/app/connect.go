package app

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/seventv/api/data/events"
	"go.uber.org/zap"

	client "github.com/seventv/eventapi/internal/app/connection"
	"github.com/seventv/eventapi/internal/global"
)

func (s *Server) TrackConnection(gctx global.Context, r *http.Request, con client.Connection) {
	if con == nil {
		return
	}

	<-con.OnReady() // wait for connection to be ready

	start := time.Now()

	// Increment counters
	atomic.AddInt32(s.activeConns, 1)

	gctx.Inst().Monitoring.EventV3().CurrentConnections.Inc()
	gctx.Inst().Monitoring.EventV3().TotalConnections.Observe(1)

	clientAddr := r.RemoteAddr

	zap.S().Debugw("new connection",
		"client_addr", clientAddr,
		"connection_count", atomic.LoadInt32(s.activeConns),
	)

	// Handle shutdown
	go func() {
		select {
		case <-s.shutdown:
			con.SendClose(events.CloseCodeRestart, 0)
		case <-con.Context().Done():
			return
		}
	}()

	<-con.OnClose() // wait for connection to end

	// Decrement counters
	atomic.AddInt32(s.activeConns, -1)

	gctx.Inst().Monitoring.EventV3().CurrentConnections.Dec()
	gctx.Inst().Monitoring.EventV3().TotalConnections.Observe(float64(time.Since(start)/time.Millisecond) / 1000)

	zap.S().Debugw("connection ended",
		"client_addr", clientAddr,
		"connection_count", atomic.LoadInt32(s.activeConns),
	)
}
