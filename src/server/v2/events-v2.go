package v2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/gofiber/fiber/v2"
)

type Session struct {
	ID      []rune
	w       *bufio.Writer
	Intents EventIntents
}

func EventsV2(app fiber.Router, start, done func()) {
	api := app.Group("/v2")

	api.Get("/", func(c *fiber.Ctx) error {
		var session Session
		// Retrieve initial intents, if any is specified
		// or it will default to 0 (no intents)
		qIntents := c.Query("intents", "0")
		intents, err := strconv.Atoi(qIntents)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(fmt.Sprintf("Bad Intents: %v", err.Error()))
		}

		// We have 2 contexts we need to respect, so we have to make a third to combine them.
		ctx := c.Context()
		usrCtx := c.UserContext()

		localCtx, cancel := context.WithCancel(context.Background())
		eventCh := make(chan string)
		mutateCh := make(chan string)
		var sid []rune

		go func() {
			start()
			defer func() {
				cancel()
				done()
				close(eventCh)
				close(mutateCh)
			}()
			select {
			case <-ctx.Done():
			case <-usrCtx.Done():
			case <-localCtx.Done():
			}
		}()

		// Listen for session mutations
		redis.Subscribe(localCtx, mutateCh, fmt.Sprintf("events-v2:mutate:%v", session.ID))

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

			// Ready the session
			{
				// Generate and assign a session ID
				sid = generateSessionID()
				session = Session{sid, w, EventIntents(intents)}

				// Write the payload
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
				case msg = <-eventCh:
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

	MutateSession(api)
}

// generateSessionID: Create a new pseudo-random session ID
func generateSessionID() []rune {
	b := make([]rune, sessionIdLength)
	for i := 0; i < sessionIdLength; i++ {
		b[i] = sessionIdRunes[rand.Intn(len(sessionIdRunes))]
	}

	return b
}

func init() {
	// Set seeding
	rand.Seed(time.Now().UnixNano())
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

type EventIntents int32

const (
	EventIntentsChannelEmote EventIntents = 1 << iota
	EventIntentsEmote
	EventIntentsUser
	EventIntentsBan
	EventIntentsApp
	EventIntentsEntitlement

	EventIntentsAll EventIntents = (1 << iota) - 1
)
