package instance

import (
	"context"
	"sync"

	goRedis "github.com/go-redis/redis/v8"
	"github.com/seventv/common/redis"
	"go.uber.org/zap"
)

type Redis interface {
	redis.Instance
	EventsSubscribe(ctx context.Context, ch chan *string, wg *sync.WaitGroup, subscribeTo ...string)
}

type RedisInst struct {
	redis.Instance
	sub     *goRedis.PubSub
	subsMtx sync.Mutex
	subs    map[string][]*redisSub
}

func WrapRedis(r redis.Instance) Redis {
	inst := &RedisInst{
		Instance: r,
		sub:      r.RawClient().Subscribe(context.Background()),
		subs:     map[string][]*redisSub{},
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				zap.S().Fatalw("panic in subs", "error", err)
			}
		}()
		ch := inst.sub.Channel()
		var msg *goRedis.Message
		for {
			msg = <-ch
			payload := msg.Payload
			inst.subsMtx.Lock()
			for _, s := range inst.subs[msg.Channel] {
				select {
				case s.ch <- &payload: // we do not want to copy the memory here so we pass a pointer
				default:
					zap.S().Debug("channel blocked dropping message: ", msg.Channel)
				}
			}
			inst.subsMtx.Unlock()
		}
	}()

	return inst
}

type redisSub struct {
	ch chan *string
}

// Subscribe to a channel on Redis
func (r *RedisInst) EventsSubscribe(ctx context.Context, ch chan *string, wg *sync.WaitGroup, subscribeTo ...string) {
	wg.Add(1)

	r.subsMtx.Lock()
	defer r.subsMtx.Unlock()
	localSub := &redisSub{ch}
	for _, e := range subscribeTo {
		if _, ok := r.subs[e]; !ok {
			_ = r.sub.Subscribe(ctx, e)
		}
		r.subs[e] = append(r.subs[e], localSub)
	}

	go func() {
		<-ctx.Done()
		r.subsMtx.Lock()
		defer r.subsMtx.Unlock()
		defer wg.Done()
		for _, e := range subscribeTo {
			for i, v := range r.subs[e] {
				if v == localSub {
					if i != len(r.subs[e])-1 {
						r.subs[e][i] = r.subs[e][len(r.subs[e])-1]
					}
					r.subs[e] = r.subs[e][:len(r.subs[e])-1]
					if len(r.subs[e]) == 0 {
						delete(r.subs, e)
						if err := r.sub.Unsubscribe(context.Background(), e); err != nil {
							zap.S().Errorw("failed to unsubscribe", "error", err)
						}
					}
					break
				}
			}
		}
	}()
}
