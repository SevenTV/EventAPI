package v1

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/SevenTV/Common/utils"
	"github.com/SevenTV/EventAPI/src/events"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/fasthttp/websocket"
	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func ChannelEmotesSSE(gCtx global.Context, ctx *fasthttp.RequestCtx) {
	channels := ctx.QueryArgs().PeekMulti("channel")
	if len(channels) == 1 {
		channels = bytes.Split(channels[0], []byte{','})
	}
	if len(channels) == 1 {
		channels = bytes.Split(channels[0], []byte{'+'})
	}
	if len(channels) == 1 {
		channels = bytes.Split(channels[0], []byte{' '})
	}
	if len(channels) > 100 || len(channels) == 0 {
		ctx.SetStatusCode(400)
		return
	}

	uniqueChannels := map[string]bool{}
	for _, c := range channels {
		uniqueChannels[strings.ToLower(utils.B2S(c))] = true
	}

	localCtx, cancel := context.WithCancel(gCtx)
	subCh := make(chan string, 10)

	start := time.Now()
	gCtx.Inst().Monitoring.EventV1().ChannelEmotes.CurrentConnections.Inc()
	gCtx.Inst().Monitoring.EventV1().ChannelEmotes.TotalConnections.Observe(1)

	go func() {
		defer func() {
			cancel()
			close(subCh)
			gCtx.Inst().Monitoring.EventV1().ChannelEmotes.CurrentConnections.Dec()
			gCtx.Inst().Monitoring.EventV1().ChannelEmotes.TotalConnectionDurationSeconds.Observe(float64(time.Since(start)/time.Millisecond) / 1000)
		}()
		select {
		case <-ctx.Done():
		case <-gCtx.Done():
		case <-localCtx.Done():
		}
	}()

	for channel := range uniqueChannels {
		gCtx.Inst().Redis.EventsSubscribe(localCtx, subCh, fmt.Sprintf("events-v1:channel-emotes:%v", channel))
	}

	events.NewEventStream(ctx, func(w *bufio.Writer) {
		conn := ctx.Conn().(*net.TCPConn)

		tick := time.NewTicker(time.Second * 30)
		defer func() {
			defer cancel()
			tick.Stop()
		}()
		var (
			msg string
			err error
		)

		if _, err = w.Write(utils.S2B("event: ready\ndata: 7tv-event-sub.v1\n\n")); err != nil {
			logrus.Error(err)
			return
		}
		if err := w.Flush(); err != nil {
			return
		}

		for {
			if err := connCheck(conn); err != nil {
				return
			}
			select {
			case <-localCtx.Done():
				return
			case <-tick.C:
				if _, err = w.Write(utils.S2B("event: heartbeat\ndata: {}\n\n")); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			case msg = <-subCh:
				if _, err = w.Write(utils.S2B(fmt.Sprintf("event: update\ndata: %s\n\n", msg))); err != nil {
					return
				}
				if err := w.Flush(); err != nil {
					return
				}
			}
		}
	})
}

type WsMessage struct {
	Action      string `json:"action"`
	Payload     string `json:"payload"`
	ClientNonce string `json:"client-nonce,omitempty"`
}

func ChannelEmotesWS(gCtx global.Context, conn *websocket.Conn) {
	localCtx, cancel := context.WithCancel(gCtx)
	subCh := make(chan string, 10)

	start := time.Now()

	go func() {
		defer func() {
			cancel()
			close(subCh)
			_ = conn.Close()
			gCtx.Inst().Monitoring.EventV1().ChannelEmotes.CurrentConnections.Dec()
			gCtx.Inst().Monitoring.EventV1().ChannelEmotes.TotalConnectionDurationSeconds.Observe(float64(time.Since(start)/time.Millisecond) / 1000)
		}()
		select {
		case <-gCtx.Done():
		case <-localCtx.Done():
		}
	}()

	gCtx.Inst().Monitoring.EventV1().ChannelEmotes.CurrentConnections.Inc()
	gCtx.Inst().Monitoring.EventV1().ChannelEmotes.TotalConnections.Observe(1)

	joinedChannels := map[string]context.CancelFunc{}
	joinedChannelsMtx := sync.Mutex{}

	writeMtx := sync.Mutex{}
	write := func(msg WsMessage) error {
		writeMtx.Lock()
		defer writeMtx.Unlock()
		return conn.WriteJSON(msg)
	}

	go func() {
		var (
			data []byte
			err  error
			msg  WsMessage
		)
		defer func() {
			cancel()
		}()
	loop:
		for {
			_, data, err = conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}

			if err := json.Unmarshal(data, &msg); err != nil {
				return
			}

			switch msg.Action {
			case "join":
				channels := strings.Split(msg.Payload, ",")
				if len(channels) == 1 {
					channels = strings.Split(channels[0], "+")
				}
				if len(channels) == 1 {
					channels = strings.Split(channels[0], " ")
				}

				uniqueChannels := map[string]bool{}
				for _, c := range channels {
					uniqueChannels[strings.ToLower(c)] = true
				}

				joinedChannelsMtx.Lock()
				if len(uniqueChannels)+len(joinedChannels) > 100 || len(uniqueChannels)+len(joinedChannels) == 0 {
					msg.Payload = "too many channels joined"
					msg.Action = "error"
					joinedChannelsMtx.Unlock()
					if err := write(msg); err != nil {
						return
					}
					continue loop
				}
				msg.Payload = strings.ToLower(msg.Payload)

				for v := range uniqueChannels {
					if _, ok := joinedChannels[v]; !ok {
						ctx, cancel := context.WithCancel(localCtx)
						gCtx.Inst().Redis.EventsSubscribe(ctx, subCh, fmt.Sprintf("events-v1:channel-emotes:%v", v))
						joinedChannels[v] = cancel
					}
				}

				joinedChannelsMtx.Unlock()
				msg.Payload = msg.Action
				msg.Action = "success"
				if err := write(msg); err != nil {
					return
				}
				continue loop
			case "part":
				channels := strings.Split(msg.Payload, ",")
				if len(channels) == 1 {
					channels = strings.Split(channels[0], "+")
				}
				if len(channels) == 1 {
					channels = strings.Split(channels[0], " ")
				}

				uniqueChannels := map[string]bool{}
				for _, c := range channels {
					uniqueChannels[strings.ToLower(c)] = true
				}

				joinedChannelsMtx.Lock()
				msg.Payload = strings.ToLower(msg.Payload)

				for v := range uniqueChannels {
					if cancel, ok := joinedChannels[v]; ok {
						cancel()
						delete(joinedChannels, v)
					}
				}

				joinedChannelsMtx.Unlock()
				msg.Payload = msg.Action
				msg.Action = "success"
				if err := write(msg); err != nil {
					return
				}
				continue loop
			default:
				msg.Payload = msg.Action
				msg.Action = "unknown"
				if err := write(msg); err != nil {
					return
				}
				continue loop
			}
		}
	}()

	tick := time.NewTicker(time.Second * 30)

	var msg string
	for {
		select {
		case <-localCtx.Done():
			return
		case <-tick.C:
			if err := write(WsMessage{
				Action: "ping",
			}); err != nil {
				return
			}
		case msg = <-subCh:
			if err := write(WsMessage{
				Action:  "update",
				Payload: msg,
			}); err != nil {
				return
			}
		}
	}
}

func connCheck(conn net.Conn) error {
	var sysErr error = nil
	rc, err := conn.(syscall.Conn).SyscallConn()
	if err != nil {
		return err
	}
	err = rc.Read(func(fd uintptr) bool {
		var buf []byte = []byte{0}
		n, _, err := syscall.Recvfrom(int(fd), buf, syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		switch {
		case n == 0 && err == nil:
			sysErr = io.EOF
		case err == syscall.EAGAIN || err == syscall.EWOULDBLOCK:
			sysErr = nil
		default:
			sysErr = err
		}
		return true
	})
	if err != nil {
		return err
	}

	return sysErr
}
