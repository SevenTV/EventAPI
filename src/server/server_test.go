package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/alicebob/miniredis/v2"
)

func InitRedis(t *testing.T) *miniredis.Miniredis {
	defer time.Sleep(time.Second)
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

func Test_EventsSingleChannel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	mr := InitRedis(t)
	defer func() {
		if mr != nil {
			mr.Close()
		}
	}()

	_, s := New(ctx, "tcp", ":3000")

	req, err := http.NewRequest("GET", "http://localhost:3000/public/v1/channel-emotes?channel=troydota", nil)
	Assert(t, err, nil, "req error")

	resp, err := http.DefaultClient.Do(req)
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 200, "response status")

	reader := bufio.NewReader(resp.Body)

	readMessage := func() string {
		msg := bytes.NewBufferString("")

		var b byte
		for {
			b, err = reader.ReadByte()
			Assert(t, err, nil, "read error")
			nextBytes, err := reader.Peek(2)
			Assert(t, err, nil, "read error")
			_ = msg.WriteByte(b)
			if nextBytes[0] == nextBytes[1] && nextBytes[0] == '\n' {
				_, _ = reader.ReadByte()
				_, _ = reader.ReadByte()
				break
			}
		}

		return msg.String()
	}

	Assert(t, err, nil, "header error")
	Assert(t, readMessage(), "event: ready\ndata: 7tv-event-sub.v1", "header value")

	testData := `{
  "channel": "troydota",
  "emote_id": "60bf2b5b74461cf8fe2d187f",
  "name": "WIDEGIGACHAD",
  "action": "ADD",
  "actor": "TroyDota",
  "emote": {
    "name": "WIDEGIGACHAD",
    "visibility": 0,
    "mime": "image/webp",
    "tags": [],
    "width": [384, 228, 144, 96],
    "height": [128, 76, 48, 32],
    "animated": true,
    "owner": {
      "id": "60af9846ebfcf7562edb10a3",
      "display_name": "noodlemanhours",
      "twitch_id": "84181465",
      "login": "noodlemanhours"
    },
  }
}`
	redis.Client.Publish(ctx, "events-v1:channel-emotes:troydota", testData)

	Assert(t, readMessage(), fmt.Sprintf("event: update\ndata: %s", testData), "data error")

	Assert(t, resp.Body.Close(), nil, "close body error")

	cancel()
	<-s
}

func Test_EventsMultiChannels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	mr := InitRedis(t)
	defer func() {
		if mr != nil {
			mr.Close()
		}
	}()

	_, s := New(ctx, "tcp", ":3000")

	req, err := http.NewRequest("GET", "http://localhost:3000/public/v1/channel-emotes?channel=troydota&channel=anatoleam", nil)
	Assert(t, err, nil, "req error")

	resp, err := http.DefaultClient.Do(req)
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 200, "response status")

	reader := bufio.NewReader(resp.Body)

	readMessage := func() string {
		msg := bytes.NewBufferString("")

		var b byte
		for {
			b, err = reader.ReadByte()
			Assert(t, err, nil, "read error")
			nextBytes, err := reader.Peek(2)
			Assert(t, err, nil, "read error")
			_ = msg.WriteByte(b)
			if nextBytes[0] == nextBytes[1] && nextBytes[0] == '\n' {
				_, _ = reader.ReadByte()
				_, _ = reader.ReadByte()
				break
			}
		}

		return msg.String()
	}

	Assert(t, err, nil, "header error")
	Assert(t, readMessage(), "event: ready\ndata: 7tv-event-sub.v1", "header value")

	testData := `{
  "channel": "troydota",
  "emote_id": "60bf2b5b74461cf8fe2d187f",
  "name": "WIDEGIGACHAD",
  "action": "ADD",
  "actor": "TroyDota",
  "emote": {
    "name": "WIDEGIGACHAD",
    "visibility": 0,
    "mime": "image/webp",
    "tags": [],
    "width": [384, 228, 144, 96],
    "height": [128, 76, 48, 32],
    "animated": true,
    "owner": {
      "id": "60af9846ebfcf7562edb10a3",
      "display_name": "noodlemanhours",
      "twitch_id": "84181465",
      "login": "noodlemanhours"
    },
  }
}`
	redis.Client.Publish(ctx, "events-v1:channel-emotes:troydota", testData)
	Assert(t, readMessage(), fmt.Sprintf("event: update\ndata: %s", testData), "data error")
	testData = `{
  "channel": "anatoleam",
  "emote_id": "60bf2b5b74461cf8fe2d187f",
  "name": "WIDEGIGACHAD",
  "action": "ADD",
  "actor": "TroyDota",
  "emote": {
    "name": "WIDEGIGACHAD",
    "visibility": 0,
    "mime": "image/webp",
    "tags": [],
    "width": [384, 228, 144, 96],
    "height": [128, 76, 48, 32],
    "animated": true,
    "owner": {
      "id": "60af9846ebfcf7562edb10a3",
      "display_name": "noodlemanhours",
      "twitch_id": "84181465",
      "login": "noodlemanhours"
    },
  }
}`
	redis.Client.Publish(ctx, "events-v1:channel-emotes:anatoleam", testData)
	Assert(t, readMessage(), fmt.Sprintf("event: update\ndata: %s", testData), "data error")

	Assert(t, resp.Body.Close(), nil, "close body error")

	cancel()
	<-s
}

func Test_EventsMultiChannelComma(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	mr := InitRedis(t)
	defer func() {
		if mr != nil {
			mr.Close()
		}
	}()

	_, s := New(ctx, "tcp", ":3000")

	req, err := http.NewRequest("GET", "http://localhost:3000/public/v1/channel-emotes?channel=troydota,anatoleam", nil)
	Assert(t, err, nil, "req error")

	resp, err := http.DefaultClient.Do(req)
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 200, "response status")

	reader := bufio.NewReader(resp.Body)

	readMessage := func() string {
		msg := bytes.NewBufferString("")

		var b byte
		for {
			b, err = reader.ReadByte()
			Assert(t, err, nil, "read error")
			nextBytes, err := reader.Peek(2)
			Assert(t, err, nil, "read error")
			_ = msg.WriteByte(b)
			if nextBytes[0] == nextBytes[1] && nextBytes[0] == '\n' {
				_, _ = reader.ReadByte()
				_, _ = reader.ReadByte()
				break
			}
		}

		return msg.String()
	}

	Assert(t, err, nil, "header error")
	Assert(t, readMessage(), "event: ready\ndata: 7tv-event-sub.v1", "header value")

	testData := `{"channel":"troydota","emote_id":"123","name":"emote-name","action":"added","author":"troydota"}`
	redis.Client.Publish(ctx, "events-v1:channel-emotes:troydota", testData)
	Assert(t, readMessage(), fmt.Sprintf("event: update\ndata: %s", testData), "data error")
	redis.Client.Publish(ctx, "events-v1:channel-emotes:anatoleam", testData)
	Assert(t, readMessage(), fmt.Sprintf("event: update\ndata: %s", testData), "data error")

	Assert(t, resp.Body.Close(), nil, "close body error")

	cancel()
	<-s
}

func Test_EventsMultiChannelPlus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	mr := InitRedis(t)
	defer func() {
		if mr != nil {
			mr.Close()
		}
	}()

	_, s := New(ctx, "tcp", ":3000")

	req, err := http.NewRequest("GET", "http://localhost:3000/public/v1/channel-emotes?channel=troydota+anatoleam", nil)
	Assert(t, err, nil, "req error")

	resp, err := http.DefaultClient.Do(req)
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 200, "response status")

	reader := bufio.NewReader(resp.Body)

	readMessage := func() string {
		msg := bytes.NewBufferString("")

		var b byte
		for {
			b, err = reader.ReadByte()
			Assert(t, err, nil, "read error")
			nextBytes, err := reader.Peek(2)
			Assert(t, err, nil, "read error")
			_ = msg.WriteByte(b)
			if nextBytes[0] == nextBytes[1] && nextBytes[0] == '\n' {
				_, _ = reader.ReadByte()
				_, _ = reader.ReadByte()
				break
			}
		}

		return msg.String()
	}

	Assert(t, err, nil, "header error")
	Assert(t, readMessage(), "event: ready\ndata: 7tv-event-sub.v1", "header value")

	testData := `{"channel":"troydota","emote_id":"123","name":"emote-name","action":"added","author":"troydota"}`
	redis.Client.Publish(ctx, "events-v1:channel-emotes:troydota", testData)
	Assert(t, readMessage(), fmt.Sprintf("event: update\ndata: %s", testData), "data error")
	redis.Client.Publish(ctx, "events-v1:channel-emotes:anatoleam", testData)
	Assert(t, readMessage(), fmt.Sprintf("event: update\ndata: %s", testData), "data error")

	Assert(t, resp.Body.Close(), nil, "close body error")

	cancel()
	<-s
}

func Test_EventsMultiChannelSpace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	mr := InitRedis(t)
	defer func() {
		if mr != nil {
			mr.Close()
		}
	}()

	_, s := New(ctx, "tcp", ":3000")

	req, err := http.NewRequest("GET", "http://localhost:3000/public/v1/channel-emotes?channel=troydota%20anatoleam", nil)
	Assert(t, err, nil, "req error")

	resp, err := http.DefaultClient.Do(req)
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 200, "response status")

	reader := bufio.NewReader(resp.Body)

	readMessage := func() string {
		msg := bytes.NewBufferString("")

		var b byte
		for {
			b, err = reader.ReadByte()
			Assert(t, err, nil, "read error")
			nextBytes, err := reader.Peek(2)
			Assert(t, err, nil, "read error")
			_ = msg.WriteByte(b)
			if nextBytes[0] == nextBytes[1] && nextBytes[0] == '\n' {
				_, _ = reader.ReadByte()
				_, _ = reader.ReadByte()
				break
			}
		}

		return msg.String()
	}

	Assert(t, err, nil, "header error")
	Assert(t, readMessage(), "event: ready\ndata: 7tv-event-sub.v1", "header value")

	testData := `{"channel":"troydota","emote_id":"123","name":"emote-name","action":"added","author":"troydota"}`
	redis.Client.Publish(ctx, "events-v1:channel-emotes:troydota", testData)
	Assert(t, readMessage(), fmt.Sprintf("event: update\ndata: %s", testData), "data error")
	redis.Client.Publish(ctx, "events-v1:channel-emotes:anatoleam", testData)
	Assert(t, readMessage(), fmt.Sprintf("event: update\ndata: %s", testData), "data error")

	Assert(t, resp.Body.Close(), nil, "close body error")

	cancel()
	<-s
}

func Test_EventsBadChannels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	mr := InitRedis(t)
	defer func() {
		if mr != nil {
			mr.Close()
		}
	}()

	_, s := New(ctx, "tcp", ":3000")

	req, err := http.NewRequest("GET", "http://localhost:3000/public/v1/channel-emotes", nil)
	Assert(t, err, nil, "req error")

	resp, err := http.DefaultClient.Do(req)
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 400, "response status")

	query := make([]string, 101)
	for i := 0; i < 101; i++ {
		query[i] = fmt.Sprintf("channel=%d", i)
	}

	req, err = http.NewRequest("GET", fmt.Sprintf("http://localhost:3000/public/v1/channel-emotes?%s", strings.Join(query, "&")), nil)
	Assert(t, err, nil, "req error")

	resp, err = http.DefaultClient.Do(req)
	Assert(t, err, nil, "response error")
	Assert(t, resp.StatusCode, 400, "response status")

	cancel()
	<-s
}

func Assert(t *testing.T, value interface{}, expected interface{}, meaning string) {
	if value != expected {
		t.Fatalf("%s, expected %v recieved %v", meaning, expected, value)
	}
}
