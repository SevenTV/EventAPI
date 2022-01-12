package v1

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/SevenTV/Common/utils"
	"github.com/SevenTV/EventAPI/src/global"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

func ChannelEmotes(gCtx global.Context, ctx *fasthttp.RequestCtx) {
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

	go func() {
		defer func() {
			cancel()
			close(subCh)
		}()
		select {
		case <-ctx.Done():
		case <-gCtx.Done():
		case <-localCtx.Done():
		}
	}()

	for channel := range uniqueChannels {
		gCtx.Inst().Redis.Subscribe(localCtx, subCh, fmt.Sprintf("events-v1:channel-emotes:%v", channel))
	}

	gCtx.Inst().Monitoring.EventV1().ChannelEmotes.CurrentConnections.Inc()
	gCtx.Inst().Monitoring.EventV1().ChannelEmotes.TotalConnections.Observe(1)

	start := time.Now()

	ctx.Response.Header.Set("Content-Type", "text/event-stream")
	ctx.Response.Header.Set("Cache-Control", "no-cache")
	ctx.Response.Header.Set("Transfer-Encoding", "chunked")
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Cache-Control")
	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	ctx.Response.Header.Set("X-Accel-Buffering", "no")

	ctx.SetStatusCode(200)

	ctx.Response.ImmediateHeaderFlush = true
	ctx.Response.SetConnectionClose()

	ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
		conn := ctx.Conn().(*net.TCPConn)

		tick := time.NewTicker(time.Second * 30)
		defer func() {
			defer cancel()
			gCtx.Inst().Monitoring.EventV1().ChannelEmotes.CurrentConnections.Dec()
			gCtx.Inst().Monitoring.EventV1().ChannelEmotes.TotalConnectionDurationSeconds.Observe(float64(time.Since(start)/time.Millisecond) / 1000)
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
