package v2

import (
	"bufio"
	"context"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
)

func EventsV2(app fiber.Router, start, done func()) {
	api := app.Group("/v2")

	api.Get("/", func(c *fiber.Ctx) error {
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
				defer cancel()
				_ = w.Flush()
				tick.Stop()
			}()
			var (
				msg string
				err error
			)

			// Write ready payload
			{
				sid := generateSessionID()
				b, err := json.Marshal(&ReadyPayload{
					Endpoint:  "7tv-event-sub.v2",
					SessionID: string(sid),
				})
				if err != nil {
					return
				}

				if _, err = w.WriteString("event: ready\ndata: "); err != nil {
					return
				}
				if _, err := w.Write(b); err != nil {
					return
				}
				if _, err := w.WriteString("\n\n"); err != nil {
					return
				}
				if err = w.Flush(); err != nil {
					return
				}
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

// generateSessionID: Create a new pseudo-random session ID
func generateSessionID() []rune {
	// Set seeding
	rand.Seed(time.Now().UnixNano())

	b := make([]rune, sessionIdLength)
	for i := 0; i < sessionIdLength; i++ {
		b[i] = sessionIdRunes[rand.Intn(len(sessionIdRunes))]
	}

	return b
}

var sessionIdRunes = []rune("abcdefghijklmnopqrstuvwxyz123456789")

const sessionIdLength = 16

type ReadyPayload struct {
	Endpoint  string `json:"endpoint"`
	SessionID string `json:"session_id"`
}

type DataPayload struct {
	// The sequence is used to resume a session if it previously disconnected,
	// recovering the events missed
	Sequence int32 `json:"seq"`
	// The type of the event
	Type string `json:"t"`
	// A json payload containing the data for the event
	Data json.RawMessage `json:"d"`
}

type EventName string

var (
	// Channel Emote Events

	EventNameChannelEmoteAdd    EventName = "CHANNEL_EMOTE_ADD"    // Channel Emote Enabled in the channel
	EventNameChannelEmoteUpdate EventName = "CHANNEL_EMOTE_UPDATE" // Channel Emote Overrides Updated for the channel
	EventNameChannelEmoteRemove EventName = "CHANNEL_EMOTE_REMOVE" // Channel Emote Disabled in the channel

	// Emote Events

	EventNameEmoteCreate EventName = "EMOTE_CREATE" // Emote Created
	EventNameEmoteEdit   EventName = "EMOTE_EDIT"   // Emote Updated
	EventNameEmoteDelete EventName = "EMOTE_DELETE" // Emote Deleted

	// User Events

	EventNameUserCreate EventName = "USER_CREATE" // User Created
	EventNameUserEdit   EventName = "USER_EDIT"   // User Updated
	EventNameUserDelete EventName = "USER_DELETE" // User Deleted

	// Ban Events

	EventNameBanCreate EventName = "BAN_CREATE" // A ban is created
	EventNameBanExpire EventName = "BAN_EXPIRE" // A ban has wore off

	// App Events

	EventNameSystemMessage EventName = "APP_SYSTEM_MESSAGE" // System message - a message broadcasted by the server that clients should display to the end user
	EventNameMetaUpdate    EventName = "APP_META_UPDATE"    // Meta Update - app meta data was updated (i.e featured broadcast changed)

	// Entitlement Events

	EventNameEntitlementCreate EventName = "ENTITLEMENT_CREATE" // Entitlement Created
	EventNameEntitlementUpdate EventName = "ENTITLEMENT_UPDATE" // Entitlement Updated
	EventNameEntitlementDelete EventName = "ENTITLEMENT_DELETE" // Entitlement Deleted
)
