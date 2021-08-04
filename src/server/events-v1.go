package server

import (
	"bufio"
	"context"
	"fmt"
	"strings"

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
		if len(query.Channels) > 100 || len(query.Channels) == 0 {
			return c.SendStatus(400)
		}

		uniqueChannels := map[string]bool{}
		for _, c := range query.Channels {
			uniqueChannels[strings.ToLower(c)] = true
		}

		resp := c.Response()

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
			redis.Subscribe(localCtx, subCh, fmt.Sprintf("users:%v:emotes", channel))
		}

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")

		resp.SetBodyStreamWriter(func(w *bufio.Writer) {
			defer func() {
				_ = w.Flush()
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
