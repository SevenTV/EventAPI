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

func NewDigest[P events.AnyPayload](gctx global.Context, op events.Opcode) *Digest[P] {
	ch := make(chan string, 10)
	d := &Digest[P]{
		gctx,
		op,
		ch,
		sync_map.Map[string, digestSub[P]]{},
	}

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
					value.ch <- o
					return true
				})
			}
		}
	}()

	return d
}

// Dispatch implements Digest
func (d *Digest[P]) Subscribe(ctx context.Context, sessionID []byte, ch chan events.Message[P]) {
	d.subs.Store(hex.EncodeToString(sessionID), digestSub[P]{ch})
}

type EventDigest struct {
	Dispatch *Digest[events.DispatchPayload]
}

type digestSub[P events.AnyPayload] struct {
	ch chan events.Message[P]
}
