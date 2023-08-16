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
	EventsSubscribe(ctx context.Context, ch chan *string, subscribeTo ...string) chan struct{}
	Unsubscribe(ch chan *string, subscribeTo ...string)
	RemoveChannel(ch chan *string)
}

type RedisInst struct {
	redis.Instance
	sub     *goRedis.PubSub
	subsMtx *sync.Mutex
	subs    map[string][]*redisSub
}

func WrapRedis(r redis.Instance) Redis {
	inst := &RedisInst{
		Instance: r,
		sub:      r.RawClient().Subscribe(context.Background()),
		subs:     map[string][]*redisSub{},
		subsMtx:  &sync.Mutex{},
	}
	go func() {
		ch := inst.sub.Channel()
		var msg *goRedis.Message
		for {
			msg = <-ch
			payload := msg.Payload
			inst.subsMtx.Lock()

			// loop over subs in reverse, so we can remove elements while iterating
			for i := len(inst.subs[msg.Channel]) - 1; i >= 0; i-- {
				sub := inst.subs[msg.Channel][i]
				select {
				case <-sub.close:
					inst.eventsUnsubscribe(sub, msg.Channel)
					// check if the string channel is closed
					select {
					case _, ok := <-sub.ch:
						if !ok {
							// channel is already closed
							continue
						}
					default:
					}
					close(sub.ch)
					continue
				default:
				}

				select {
				case sub.ch <- &payload: // we do not want to copy the memory here so we pass a pointer
				default:
					zap.S().Debug("channel blocked dropping message: ", msg.Channel)
					// TODO: close channel, on receiving end drop connection
				}
			}

			inst.subsMtx.Unlock()
		}
	}()

	return inst
}

type redisSub struct {
	ch    chan *string
	close chan struct{}
}

// Subscribe to a channel on Redis
func (r *RedisInst) EventsSubscribe(ctx context.Context, ch chan *string, subscribeTo ...string) chan struct{} {
	r.subsMtx.Lock()
	defer r.subsMtx.Unlock()
	sub := &redisSub{ch: ch, close: make(chan struct{})}
	for _, e := range subscribeTo {
		if _, ok := r.subs[e]; !ok {
			_ = r.sub.Subscribe(ctx, e)
		}
		r.subs[e] = append(r.subs[e], sub)
	}
	return sub.close
}

// Unsubscribe from a channel on Redis
func (r *RedisInst) eventsUnsubscribe(rsub *redisSub, subscribeTo ...string) {
	for _, sub := range subscribeTo {
		for i, v := range r.subs[sub] {
			if v != rsub {
				continue
			}
			r.subs[sub] = removeRedisSub(r.subs[sub], i)
			if len(r.subs[sub]) == 0 {
				delete(r.subs, sub)
				if err := r.sub.Unsubscribe(context.Background(), sub); err != nil {
					zap.S().Errorw("failed to unsubscribe", "error", err)
				}
			}
			break
		}
	}
}

// Unsubscribe from a channel on Redis
func (r *RedisInst) Unsubscribe(ch chan *string, subscribeTo ...string) {
	r.subsMtx.Lock()
	defer r.subsMtx.Unlock()

	for _, sub := range subscribeTo {
		for i, v := range r.subs[sub] {
			if v.ch != ch {
				continue
			}
			r.subs[sub] = removeRedisSub(r.subs[sub], i)
			if len(r.subs[sub]) == 0 {
				delete(r.subs, sub)
				if err := r.sub.Unsubscribe(context.Background(), sub); err != nil {
					zap.S().Errorw("failed to unsubscribe", "error", err)
				}
			}
			break
		}
	}
}

// RemoveChannel removes all subscriptions with the given channel, it is used only for v1 backwards compatibility
func (r *RedisInst) RemoveChannel(ch chan *string) {
	r.subsMtx.Lock()
	defer r.subsMtx.Unlock()

	for sub, slice := range r.subs {
		for i, v := range slice {
			if v.ch != ch {
				continue
			}
			r.subs[sub] = removeRedisSub(slice, i)
			if len(r.subs[sub]) == 0 {
				delete(r.subs, sub)
				if err := r.sub.Unsubscribe(context.Background(), sub); err != nil {
					zap.S().Errorw("failed to unsubscribe", "error", err)
				}
			}
			break
		}
	}
}

func removeRedisSub(slice []*redisSub, i int) []*redisSub {
	slice[i] = slice[len(slice)-1]
	return slice[:len(slice)-1]
}
