package redis

import (
	"context"

	"github.com/sirupsen/logrus"
)

// Publish to a redis channel
func Publish(ctx context.Context, channel string, data string) error {
	return Client.Publish(ctx, channel, data).Err()
}

// Subscribe to a channel on Redis
func Subscribe(ctx context.Context, ch chan string, subscribeTo ...string) {
	subsMtx.Lock()
	defer subsMtx.Unlock()
	localSub := &redisSub{ch}
	for _, e := range subscribeTo {
		if _, ok := subs[e]; !ok {
			_ = sub.Subscribe(ctx, e)
		}
		subs[e] = append(subs[e], localSub)
	}

	go func() {
		<-ctx.Done()
		subsMtx.Lock()
		defer subsMtx.Unlock()
		for _, e := range subscribeTo {
			for i, v := range subs[e] {
				if v == localSub {
					if i != len(subs[e])-1 {
						subs[e][i] = subs[e][len(subs[e])-1]
					}
					subs[e] = subs[e][:len(subs[e])-1]
					if len(subs[e]) == 0 {
						delete(subs, e)
						if err := sub.Unsubscribe(context.Background(), e); err != nil {
							logrus.WithError(err).Error("failed to unsubscribe")
						}
					}
					break
				}
			}
		}
	}()
}
