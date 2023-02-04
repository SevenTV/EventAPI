package client

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"sync"

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
	ch := make(chan string, 1024)
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
					o := events.Message[P]{}
					if err = json.Unmarshal(utils.S2B(s), &o); err != nil {
						zap.S().Warnw("got badly encoded message",
							"error", err.Error(),
						)
					}

					d.subs.Range(func(key string, value *DigestSub[P]) bool {
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

// Dispatch implements Digest
func (d *Digest[P]) Subscribe(ctx context.Context, sessionID []byte, size int) <-chan events.Message[P] {
	sid := hex.EncodeToString(sessionID)

	ds := &DigestSub[P]{make(chan events.Message[P], size), sync.Once{}}

	d.subs.Store(sid, ds)

	go func() {
		<-ctx.Done()
		ds.close()
		d.subs.Delete(sid)
	}()

	return ds.ch
}

type EventDigest struct {
	Dispatch *Digest[events.DispatchPayload]
	Ack      *Digest[events.AckPayload]
}

type DigestSub[P events.AnyPayload] struct {
	ch   chan events.Message[P]
	once sync.Once
}

func (ds *DigestSub[P]) close() {
	ds.once.Do(func() {
		close(ds.ch)
	})
}
