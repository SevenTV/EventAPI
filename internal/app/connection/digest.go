package client

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/seventv/api/data/events"
	"github.com/seventv/common/redis"
	"github.com/seventv/common/sync_map"
	"github.com/seventv/common/utils"
	"github.com/seventv/eventapi/internal/global"
	"go.uber.org/zap"
)

type Digest[P events.AnyPayload] struct {
	gctx global.Context
	ch   chan string
	subs *sync_map.Map[string, *DigestSub[P]]
}

func NewDigest[P events.AnyPayload](gctx global.Context, key redis.Key) *Digest[P] {
	ch := make(chan string, 16384)
	d := &Digest[P]{
		gctx: gctx,
		ch:   ch,
		subs: &sync_map.Map[string, *DigestSub[P]]{},
	}

	if key != "" {
		go gctx.Inst().Redis.Subscribe(gctx, ch, key)
		go func() {
			defer close(ch)

			var (
				s   string
				err error
			)
			for {
				select {
				case <-gctx.Done():
					return
				case s = <-ch:
					var o events.Message[json.RawMessage]
					if err = json.Unmarshal(utils.S2B(s), &o); err != nil {
						zap.S().Warnw("got badly encoded payload",
							"error", err.Error(),
						)
						continue
					}

					var asDispatch events.Message[events.DispatchPayload]

					switch o.Op {
					case events.OpcodeDispatch:
						asDispatch, err = events.ConvertMessage[events.DispatchPayload](o)
					}

					if err != nil {
						zap.S().Warnw("got badly encoded payload",
							"opcode", o.Op,
							"error", err.Error(),
						)
					}

					d.subs.Range(func(key string, value *DigestSub[P]) bool {
						switch o.Op {
						case events.OpcodeDispatch:
							value.conn.Handler().OnDispatch(gctx, asDispatch)
						}

						return true
					})
				}
			}
		}()
	}

	return d
}

// Dispatch implements Digest
func (d *Digest[P]) Subscribe(ctx context.Context, conn Connection, sessionID []byte) {
	sid := hex.EncodeToString(sessionID)

	ds := &DigestSub[P]{conn}

	d.subs.Store(sid, ds)

	go func() {
		<-ctx.Done()
		d.subs.Delete(sid)
	}()
}

type EventDigest struct {
	Dispatch *Digest[events.DispatchPayload]
}

type DigestSub[P events.AnyPayload] struct {
	conn Connection
}
