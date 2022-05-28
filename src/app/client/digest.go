package client

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/SevenTV/Common/events"
	"github.com/SevenTV/Common/redis"
	"github.com/SevenTV/Common/sync_map"
	"github.com/SevenTV/Common/utils"
	"github.com/SevenTV/EventAPI/src/global"
	"go.uber.org/zap"
)

type Digest[P events.AnyPayload] struct {
	gctx global.Context
	op   events.Opcode
	ch   chan string
	subs sync_map.Map[string, digestSub[P]]
}

func NewDigest[P events.AnyPayload](gctx global.Context, op events.Opcode, useRedis bool) *Digest[P] {
	ch := make(chan string, 10)
	d := &Digest[P]{
		gctx,
		op,
		ch,
		sync_map.Map[string, digestSub[P]]{},
	}

	if useRedis {
		go gctx.Inst().Redis.Subscribe(gctx, ch, redis.Key(op.PublishKey()))
		go func() {
			defer close(ch)

			var (
				s   string
				err error
				o   events.Message[P]
			)
			for {
				select {
				case <-gctx.Done():
					return
				case s = <-ch:
					if err = json.Unmarshal(utils.S2B(s), &o); err != nil {
						zap.S().Warnw("got badly encoded message",
							"error", err.Error(),
						)
					}

					d.subs.Range(func(key string, value digestSub[P]) bool {
						select {
						case value.ch <- o:
						default:
							zap.S().Warnw("channel blocked",
								"channel", key,
							)
						}
						return true
					})
				}
			}
		}()
	}
	return d
}

func (d *Digest[P]) Publish(ctx context.Context, msg events.Message[json.RawMessage]) {
	m, err := events.ConvertMessage[P](msg)
	if err == nil {
		d.subs.Range(func(key string, value digestSub[P]) bool {
			value.ch <- m
			return true
		})
	}
}

// Dispatch implements Digest
func (d *Digest[P]) Subscribe(ctx context.Context, sessionID []byte, ch chan events.Message[P]) {
	sid := hex.EncodeToString(sessionID)
	d.subs.Store(sid, digestSub[P]{ch})
	<-ctx.Done()
	d.subs.Delete(sid)
}

type EventDigest struct {
	Dispatch *Digest[events.DispatchPayload]
}

type digestSub[P events.AnyPayload] struct {
	ch chan events.Message[P]
}
