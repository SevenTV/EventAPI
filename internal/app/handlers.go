package app

import (
	"net/http"
	"strings"

	"go.uber.org/zap"

	client "github.com/seventv/eventapi/internal/app/connection"
	v3 "github.com/seventv/eventapi/internal/app/v3"
)

func writeBytesResponse(code int, res []byte, w http.ResponseWriter) {
	w.WriteHeader(code)
	_, err := w.Write(res)
	if err != nil {
		zap.S().Errorw("failed to write http response", "error", err)
	}
}

func writeError(code int, e error, w http.ResponseWriter) {
	w.WriteHeader(code)
	_, err := w.Write([]byte(e.Error()))
	if err != nil {
		zap.S().Errorw("failed to write http response", "error", err)
	}
}

func (s *Server) handleV3(w http.ResponseWriter, r *http.Request) {
	if !s.gctx.Config().API.V3 {
		writeBytesResponse(http.StatusServiceUnavailable, []byte("Service unavailable"), w)
		return
	}

	var (
		con client.Connection
	)

	connected := false

	if strings.ToLower(r.Header.Get("upgrade")) == "websocket" || strings.ToLower(r.Header.Get("connection")) == "upgrade" {
		c, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			writeError(http.StatusBadRequest, err, w)
			return
		}
		con, err = v3.WebSocket(s.gctx, c)
		if err != nil {
			writeError(http.StatusBadRequest, err, w)
			return
		}
		connected = true
	} else { // New EventStream connection
		var err error
		con, err = v3.SSE(s.gctx, w, r)
		if err != nil {
			writeError(http.StatusBadRequest, err, w)
			return
		}

		connected = true
	}

	go func() {
		if !connected {
			return
		}

		s.TrackConnection(s.gctx, r, con)
	}()
}
