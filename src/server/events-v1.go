package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/SevenTV/EventAPI/src/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

type v1Query struct {
	Channels []string `query:"channel"`
}

func EventsV1(app fiber.Router, start, done func()) {
	api := app.Group("/v1")

	api.Get("/channel-emotes", func(c *fiber.Ctx) error {
		query := v1Query{}
		if err := c.QueryParser(&query); err != nil {
			return c.SendStatus(400)
		}
		if len(query.Channels) == 1 {
			query.Channels = strings.Split(query.Channels[0], ",")
		}
		if len(query.Channels) == 1 {
			query.Channels = strings.Split(query.Channels[0], "+")
		}
		if len(query.Channels) == 1 {
			query.Channels = strings.Split(query.Channels[0], " ")
		}
		if len(query.Channels) > 100 || len(query.Channels) == 0 {
			return c.SendStatus(400)
		}

		uniqueChannels := map[string]bool{}
		for _, c := range query.Channels {
			uniqueChannels[strings.ToLower(c)] = true
		}

		// We have 2 contexts we need to respect, so we have to make a third to combine them.
		ctx := c.Context()
		usrCtx := c.UserContext()

		localCtx, cancel := context.WithCancel(context.Background())
		subCh := make(chan string)

		go func() {
			start()
			defer func() {
				cancel()
				done()
				close(subCh)
			}()
			select {
			case <-ctx.Done():
			case <-usrCtx.Done():
			case <-localCtx.Done():
			}
		}()

		for channel := range uniqueChannels {
			redis.Subscribe(localCtx, subCh, fmt.Sprintf("events-v1:channel-emotes:%v", channel))
		}

		ctx.Response.Header.Set("Content-Type", "text/event-stream")
		ctx.Response.Header.Set("Cache-Control", "no-cache")
		ctx.Response.Header.Set("Transfer-Encoding", "chunked")
		ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
		ctx.Response.Header.Set("Access-Control-Allow-Headers", "Cache-Control")
		ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		ctx.Response.Header.Set("X-Accel-Buffering", "no")

		ctx.Response.ImmediateHeaderFlush = true
		ctx.Response.SetConnectionClose()

		ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
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

		return nil
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
