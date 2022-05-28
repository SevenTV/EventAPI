package app

import (
	"github.com/SevenTV/Common/sync_map"
	"github.com/SevenTV/EventAPI/src/app/client"
	"github.com/valyala/fasthttp"
)

func SessionMutationHandler(ctx *fasthttp.RequestCtx, conns *sync_map.Map[string, client.Connection]) error {
	return nil
}
