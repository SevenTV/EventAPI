package redis

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func InitRedis(t *testing.T) *miniredis.Miniredis {
	if val, ok := os.LookupEnv("USE_LOCAL_REDIS"); ok && val == "1" {
		Init("redis://127.0.0.1:6379/0")
		return nil
	}
	mr, err := miniredis.Run()
	Assert(t, err, nil, "miniredis starts")
	Init(fmt.Sprintf("redis://%s/0", mr.Addr()))
	return mr
}

func Test_RedisPublishSubscribe(t *testing.T) {
	mr := InitRedis(t)
	defer func() {
		if mr != nil {
			mr.Close()
		}
	}()

	testData := "pog"

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	cb := make(chan string)
	defer close(cb)

	Subscribe(ctx, cb, "events:xd")
	Assert(t, Publish(ctx, "events:xd", testData), nil, "publish test")
	Assert(t, <-cb, testData, "test data")
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
