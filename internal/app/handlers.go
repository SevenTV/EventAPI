package app

import (
	"net/http"
	"strings"

	"go.uber.org/zap"

	client "github.com/seventv/eventapi/internal/app/connection"
	client_eventstream "github.com/seventv/eventapi/internal/app/connection/eventstream"
	client_websocket "github.com/seventv/eventapi/internal/app/connection/websocket"
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

	if strings.ToLower(r.Header.Get("upgrade")) == "websocket" || strings.ToLower(r.Header.Get("connection")) == "upgrade" {
		c, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			writeError(http.StatusBadRequest, err, w)
			return
		}

		con, err = client_websocket.NewWebSocket(s.gctx, c)
		if err != nil {
			writeError(http.StatusBadRequest, err, w)
			return
		}

		err = v3.WebSocket(s.gctx, con)
		if err != nil {
			writeError(http.StatusBadRequest, err, w)
			return
		}

		go s.TrackConnection(s.gctx, r, con)
	} else { // New EventStream connection
		var err error

		con, err := client_eventstream.NewEventStream(s.gctx, r)
		if err != nil {
			return
		}

		client_eventstream.SetEventStreamHeaders(w)

		go s.TrackConnection(s.gctx, r, con)

		err = v3.SSE(s.gctx, con, w, r)
		if err != nil {
			writeError(http.StatusBadRequest, err, w)
			return
		}
	}
}
