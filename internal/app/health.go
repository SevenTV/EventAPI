package app

import (
	"context"
	"net/http"
	"time"
)

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	if err := s.gctx.Inst().Redis.Ping(ctx); err != nil {
		writeBytesResponse(http.StatusServiceUnavailable, []byte("redis down"), w)
		return
	}
	writeBytesResponse(http.StatusOK, []byte("OK"), w)
}
