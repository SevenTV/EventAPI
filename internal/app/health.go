package app

import (
	"net/http"
)

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeBytesResponse(http.StatusOK, []byte("OK"), w)
}
