package app

import (
	"net/http"
	"strings"

	client "github.com/seventv/eventapi/internal/app/connection"
	v1 "github.com/seventv/eventapi/internal/app/v1"
	v3 "github.com/seventv/eventapi/internal/app/v3"
)

func writeBytesResponse(code int, res []byte, w http.ResponseWriter) error {
	w.WriteHeader(code)
	_, err := w.Write(res)
	return err
}

func writeError(code int, e error, w http.ResponseWriter) {
	w.WriteHeader(code)
	w.Write([]byte(e.Error()))
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
		// TODO: re-enable SSE & rewrite without fasthttp
		writeBytesResponse(http.StatusServiceUnavailable, []byte("Service unavailable"), w)
		return
		//con, err = v3.SSE(s.gctx, r.Context(), s.router)
		//if err != nil {
		//	writeError(http.StatusBadRequest, err, w)
		//	return
		//}
		//
		//connected <- true
	}

	go func() {
		if !connected {
			return
		}

		s.TrackConnection(s.gctx, r, con)
	}()
}

func (s *Server) handleV1(w http.ResponseWriter, r *http.Request) {
	if !s.gctx.Config().API.V1 {
		writeBytesResponse(http.StatusServiceUnavailable, []byte("Service unavailable"), w)
		return
	}

	if strings.ToLower(r.Header.Get("upgrade")) == "websocket" || strings.ToLower(r.Header.Get("connection")) == "upgrade" {
		c, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			writeError(http.StatusBadRequest, err, w)
			return
		}
		v1.ChannelEmotesWS(s.gctx, c)
	} else {
		// TODO: re-enable SSE & rewrite without fasthttp
		writeBytesResponse(http.StatusServiceUnavailable, []byte("Service unavailable"), w)

		//v1.ChannelEmotesSSE(gctx, ctx)
	}

}