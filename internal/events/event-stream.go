package events

import (
	"github.com/valyala/fasthttp"
)

func NewEventStream(ctx *fasthttp.RequestCtx, writer fasthttp.StreamWriter) {
	ctx.SetStatusCode(200)

	ctx.Response.ImmediateHeaderFlush = true
	ctx.Response.SetConnectionClose()

	ctx.Response.Header.Set("Content-Type", "text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Transfer-Encoding", "chunked")
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Cache-Control")
	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	ctx.Response.Header.Set("X-Accel-Buffering", "no")

	ctx.SetBodyStreamWriter(writer)
}
