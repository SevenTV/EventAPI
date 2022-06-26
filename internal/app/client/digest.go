package client

import (
	"context"
	"encoding/hex"
	"encoding/json"

	"github.com/seventv/common/events"
	"github.com/seventv/common/redis"
	"github.com/seventv/common/sync_map"
	"github.com/seventv/common/utils"
	"github.com/seventv/eventapi/internal/global"
	"go.uber.org/zap"
)

type Digest[P events.AnyPayload] struct {
	gctx global.Context
	ch   chan string
	subs *sync_map.Map[string, digestSub[P]]
}

func NewDigest[P events.AnyPayload](gctx global.Context, key redis.Key) *Digest[P] {
	ch := make(chan string, 10)
	d := &Digest[P]{
		gctx: gctx,
		ch:   ch,
		subs: &sync_map.Map[string, digestSub[P]]{},
	}

	if key != "" {
		go gctx.Inst().Redis.Subscribe(gctx, ch, key)
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
	Ack      *Digest[events.AckPayload]
}

type digestSub[P events.AnyPayload] struct {
	ch chan events.Message[P]
}
