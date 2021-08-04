package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func InitRedis(t *testing.T) *miniredis.Miniredis {
	mr, err := miniredis.Run()
	Assert(t, err, nil, "miniredis starts")
	Init(fmt.Sprintf("redis://%s/0", mr.Addr()))
	return mr
}

func Test_RedisPublishSubscribe(t *testing.T) {
	InitRedis(t)

	testData := "pog"

	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cb := make(chan string)
	Subscribe(ctx, cb, "events:xd")
	Subscribe(ctx, cb, "events:xd")
	Subscribe(ctx, cb, "events:xd")
	Assert(t, Publish(ctx, "events:xd", testData), nil, "publish test")
	Assert(t, <-cb, testData, "test data 1")
	Assert(t, <-cb, testData, "test data 2")
	Assert(t, <-cb, testData, "test data 3")
	cancel()

	time.Sleep(time.Second)

	subsMtx.Lock()
	Assert(t, len(subs["events:xd"]), 0, "clean up event listener")
	Assert(t, len(subs), 0, "clean up event listener")
	subsMtx.Unlock()
}

func Assert(t *testing.T, value interface{}, expected interface{}, meaning string) {
	if value != expected {
		t.Fatalf("%s, expected %v recieved %v", meaning, expected, value)
	}
}
