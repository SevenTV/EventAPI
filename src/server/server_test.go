package server

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/SevenTV/EventAPI/src/utils"
	"github.com/alicebob/miniredis/v2"
)

func InitRedis(t *testing.T) *miniredis.Miniredis {
	if val, ok := os.LookupEnv("USE_LOCAL_REDIS"); ok && val == "1" {
		redis.Init("redis://127.0.0.1:6379/0")
		return nil
	}
	mr, err := miniredis.Run()
	Assert(t, err, nil, "miniredis starts")
	redis.Init(fmt.Sprintf("redis://%s/0", mr.Addr()))
	return mr
}

func Test_Health(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	mr := InitRedis(t)
	defer func() {
		if mr != nil {
			mr.Close()
		}
	}()

	app, s := New(ctx, "tcp", ":3000")

	resp, err := app.Test(httptest.NewRequest("GET", "http://localhost:3000/health", nil))
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 200, "response status")

	cancel()
	<-s
}

func Test_PanicHandler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)

	app, s := New(ctx, "tcp", ":3000")

	resp, err := app.Test(httptest.NewRequest("GET", "http://localhost:3000/testing/panic", nil))
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 500, "response status")

	cancel()
	<-s
}

func Test_Events(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	mr := InitRedis(t)
	defer func() {
		if mr != nil {
			mr.Close()
		}
	}()

	_, s := New(ctx, "tcp", ":3000")

	req, err := http.NewRequest("GET", "http://localhost:3000/v1/troydota", nil)
	Assert(t, err, nil, "request error")
	client := http.Client{}
	resp, err := client.Do(req)
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 200, "response status")

	reader := bufio.NewReader(resp.Body)

	header := make([]byte, len(serverHeader))
	_, err = reader.Read(header)
	Assert(t, err, nil, "header error")
	Assert(t, utils.B2S(header), utils.B2S(serverHeader), "header value")

	testData := "event sub works really well"
	mr.Publish("users:troydota:emotes", testData)

	msg, err := reader.ReadBytes(0)
	Assert(t, err, nil, "read error")
	msg = msg[:len(msg)-1]
	Assert(t, utils.B2S(msg), testData, "data error")

	Assert(t, resp.Body.Close(), nil, "close body error")

	cancel()
	<-s
}

func Assert(t *testing.T, value interface{}, expected interface{}, meaning string) {
	if value != expected {
		t.Fatalf("%s, expected %v recieved %v", meaning, expected, value)
	}
}
