package util

import (
	"net"
	"net/http"

	"golang.org/x/net/context"
)

type ContextKey struct {
	key string
}

var ConnContextKey = &ContextKey{"http-conn"}

func SaveConnInContext(ctx context.Context, c net.Conn) context.Context {
	return context.WithValue(ctx, ConnContextKey, c)
}

func GetConn(r *http.Request) net.Conn {
	return r.Context().Value(ConnContextKey).(net.Conn)
}
