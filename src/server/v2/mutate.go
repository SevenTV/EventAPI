package v2

import (
	"encoding/json"
	"fmt"

	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/SevenTV/EventAPI/src/utils"
	"github.com/gofiber/fiber/v2"
	log "github.com/sirupsen/logrus"
)

func MutateSession(app fiber.Router) {
	app.Put("/:session/intents", func(c *fiber.Ctx) error {
		ctx := c.Context()
		sessionID := c.Params("session")
		if len(sessionID) != sessionIdLength {
			return c.Status(fiber.StatusBadRequest).SendString("Bad Session ID")
		}

		// Parse the payload
		var intents []sessionMutationIntent
		if err := json.Unmarshal(c.Body(), &intents); err != nil {
			return c.Status(fiber.StatusBadRequest).SendString(err.Error())
		}
		if len(intents) == 0 {
			return c.Status(fiber.StatusBadRequest).SendString("No Intents Specified")
		}

		// Send the payload
		for _, intent := range intents {
			j, err := json.Marshal(sessionMutationEvent{
				Action:     "CREATE",
				IntentName: string(intent.Intent),
				Targets:    intent.Targets,
			})
			if err != nil {
				log.WithError(err).Error("json")
				return c.SendStatus(fiber.StatusInternalServerError)
			}

			if err := redis.Publish(ctx, fmt.Sprintf("events-v2:mutate:%v", sessionID), utils.B2S(j)); err != nil {
				log.WithError(err).Error("redis")
			}

		}

		return c.Status(fiber.StatusOK).SendString("OK")
	})
}

type sessionMutationIntent struct {
	Intent  EventIntents `json:"intent"`
	Targets []string     `json:"targets"`
}

type sessionMutationEvent struct {
	Action     string   `json:"action"`
	IntentName string   `json:"intent_name"`
	Targets    []string `json:"targets,omitempty"`
}
