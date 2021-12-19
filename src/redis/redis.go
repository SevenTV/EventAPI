package redis

import (
	"context"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

func Init(uri string) {
	options, err := redis.ParseURL(uri)
	if err != nil {
		logrus.WithError(err).Fatal("redis failed")
	}

	Client = redis.NewClient(options)

	sub = Client.Subscribe(context.Background())
	go func() {
		defer func() {
			if err := recover(); err != nil {
				logrus.WithField("err", err).Fatal("panic in subs")
			}
		}()
		ch := sub.Channel()
		var msg *redis.Message
		for {
			msg = <-ch
			payload := msg.Payload // dont change we want to copy the memory due to concurrency.
			subsMtx.Lock()
			for _, s := range subs[msg.Channel] {
				go func(s *redisSub) {
					defer func() {
						if err := recover(); err != nil {
							logrus.WithField("err", err).Error("panic in subs")
						}
					}()
					s.ch <- payload
				}(s)
			}
			subsMtx.Unlock()
		}
	}()
}

var Client *redis.Client

var (
	sub     *redis.PubSub
	subs    = map[string][]*redisSub{}
	subsMtx = sync.Mutex{}
)

type redisSub struct {
	ch chan string
}

type Message = redis.Message

type StringCmd = redis.StringCmd

type StringStringMapCmd = redis.StringStringMapCmd

type PubSub = redis.PubSub

type Z = redis.Z

const ErrNil = redis.Nil
