package server

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/gofiber/fiber/v2"
)

type v1Query struct {
	Channels []string `query:"channel"`
}

func EventsV1(app fiber.Router) {
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
			defer func() {
				cancel()
				close(subCh)
			}()
			select {
			case <-ctx.Done():
			case <-usrCtx.Done():
			}
		}()

		for channel := range uniqueChannels {
			redis.Subscribe(localCtx, subCh, fmt.Sprintf("events-v1:channel-emotes:%v", channel))
		}

		ctx.SetContentType("text/event-stream")
		ctx.Response.Header.Set("Cache-Control", "no-cache")
		ctx.Response.Header.Set("Connection", "keep-alive")
		ctx.Response.Header.Set("Transfer-Encoding", "chunked")
		ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
		ctx.Response.Header.Set("Access-Control-Allow-Headers", "Cache-Control")
		ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		ctx.Response.Header.Set("X-Accel-Buffering", "no")

		ctx.SetBodyStreamWriter(func(w *bufio.Writer) {
			tick := time.NewTicker(time.Second * 30)
			defer func() {
				_ = w.Flush()
				tick.Stop()
			}()
			var (
				msg string
				err error
			)
			if _, err = w.WriteString("event: connected\ndata: 7tv-event-sub.v1\n\n"); err != nil {
				return
			}
			if err = w.Flush(); err != nil {
				return
			}
			for {
				select {
				case <-localCtx.Done():
					return
				case <-tick.C:
					if _, err = w.WriteString("event: heartbeat\n"); err != nil {
						return
					}
					if _, err = w.WriteString("data: {}\n\n"); err != nil {
						return
					}
					if err = w.Flush(); err != nil {
						return
					}
				case msg = <-subCh:
					if _, err = w.WriteString("event: update\n"); err != nil {
						return
					}
					if _, err = w.WriteString("data: "); err != nil {
						return
					}
					if _, err = w.WriteString(msg); err != nil {
						return
					}
					// Write a 0 byte to signify end of a message to signify end of event.
					if _, err = w.WriteString("\n\n"); err != nil {
						return
					}
					if err = w.Flush(); err != nil {
						return
					}
				}
			}
		})

		return nil
	})
}
