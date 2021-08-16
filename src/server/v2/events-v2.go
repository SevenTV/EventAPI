package v2

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"time"

	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/SevenTV/EventAPI/src/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

type Session struct {
	ID       []rune
	w        *bufio.Writer
	Sequence int32
	Intents  map[EventIntents]SessionIntent
}

type SessionIntent struct {
	Name    string
	Ctx     context.Context
	Cancel  context.CancelFunc
	Targets []string
}

var eventSplit = regexp.MustCompile("^[^:]+.")

func EventsV2(app fiber.Router, start, done func()) {
	api := app.Group("/v2")

	api.Get("/", func(c *fiber.Ctx) error {
		var session Session

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

				// Cancel all active intents
				for _, intent := range session.Intents {
					intent.Cancel()
				}
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
				data     string
				mutation string
				err      error
			)

			// Ready the session
			{
				// Generate and assign a session ID
				sid = generateSessionID()
				session = Session{sid, w, 0, map[EventIntents]SessionIntent{}}

				// Write the payload
				b, err := json.Marshal(&ReadyPayload{
					Endpoint:  "7tv-event-sub.v2",
					SessionID: string(sid),
					Intents:   []SessionIntent{},
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

			// Listen for session mutations
			redis.Subscribe(localCtx, mutateCh, fmt.Sprintf("events-v2:mutate:%v", string(session.ID)))

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
				// Handle an incoming event
				case data = <-eventCh:
					// Split event name | data
					s := eventSplit.FindStringSubmatch(data)
					if len(s) != 1 {
						// Log an error if the API sent an invalid payload
						log.WithFields(log.Fields{
							"session_id": session.ID,
							"seq":        session.Sequence,
						}).Error("API sent an invalid event payload")
						return
					}
					eventName := s[0][:len(s[0])-1]
					payload := data[len(eventName)+1:]

					session.Sequence++
					j, err := json.Marshal(DataPayload{
						Sequence: session.Sequence,
						Type:     EventName(eventName),
						Data:     json.RawMessage(payload),
					})
					if err != nil {
						log.WithError(err).Error("json")
						return
					}

					if _, err = w.WriteString("event: update\n"); err != nil {
						return
					}
					if _, err = w.WriteString("data: "); err != nil {
						return
					}

					if _, err = w.WriteString(string(j)); err != nil {
						return
					}
					// Write a 0 byte to signify end of a message to signify end of event.
					if _, err = w.WriteString("\n\n"); err != nil {
						return
					}
					if err = w.Flush(); err != nil {
						return
					}
				// Handle a mutation on the active session
				case mutation = <-mutateCh:
					var mutationEvent sessionMutationEvent
					if err := json.Unmarshal(utils.S2B(mutation), &mutationEvent); err != nil {
						log.WithError(err).Error("json")
						return
					}

					// Find existing intent & edit it
					intent, ok := session.Intents[EventIntents(mutationEvent.IntentName)]
					if mutationEvent.Action != "DELETE" {
						if !ok {
							intentCtx, intentCancel := context.WithCancel(context.Background())
							intent = SessionIntent{
								Name:    mutationEvent.IntentName,
								Targets: mutationEvent.Targets,
								Ctx:     intentCtx,
								Cancel:  intentCancel,
							}
							session.Intents[EventIntents(mutationEvent.IntentName)] = intent
						} else {
							// Cancel the existing subscription & create a new context
							intent.Cancel()
							intentCtx, intentCancel := context.WithCancel(context.Background())
							intent.Ctx = intentCtx
							intent.Cancel = intentCancel

							mutationEvent.Action = "UPDATE" // Set the action to update, as the intent was only modified not created
							intent.Targets = mutationEvent.Targets
						}

						// Subscribe
						for _, target := range intent.Targets {
							redis.Subscribe(intent.Ctx, eventCh, fmt.Sprintf("events-v2:%s:%s", intent.Name, target))
						}
					} else {
						if ok { // Cancel the existing subscription
							intent.Cancel()
						}
					}

					// Mutation occured
					if _, err := w.WriteString("event: mutation\n"); err != nil {
						return
					}
					if _, err := w.WriteString("data: "); err != nil {
						return
					}

					j, err := json.Marshal(map[string]interface{}{
						"action":  mutationEvent.Action,
						"intent":  mutationEvent.IntentName,
						"targets": mutationEvent.Targets,
					})
					if err != nil {
						log.WithError(err).Error("json")
					}
					if _, err := w.WriteString(string(j)); err != nil {
						return
					}
					if _, err := w.WriteString("\n\n"); err != nil {
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
	Endpoint  string          `json:"endpoint"`
	SessionID string          `json:"session_id"`
	Intents   []SessionIntent `json:"intents"`
}

type DataPayload struct {
	// The sequence is used to resume a session if it previously disconnected,
	// recovering the events missed
	Sequence int32 `json:"seq"`
	// The type of the event
	Type EventName `json:"t"`
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

type EventIntents string

const (
	EventIntentsChannelEmote EventIntents = "channel-emote"
	EventIntentsEmote        EventIntents = "emote"
	EventIntentsUser         EventIntents = "user"
	EventIntentsBan          EventIntents = "ban"
	EventIntentsApp          EventIntents = "app"
	EventIntentsEntitlement  EventIntents = "entitlement"
)
