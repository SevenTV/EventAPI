package server

import (
	"context"
	"sync"
	"time"

	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/gofiber/fiber/v2"

	log "github.com/sirupsen/logrus"
)

func Health(app fiber.Router) {
	mtx := sync.Mutex{}

	app.Get("/health", func(c *fiber.Ctx) error {
		mtx.Lock()
		defer mtx.Unlock()

		redisCtx, cancel := context.WithTimeout(c.Context(), time.Second*10)
		defer cancel()
		if err := redis.Client.Ping(redisCtx).Err(); err != nil {
			log.WithError(err).Error("health, REDIS IS DOWN")
			return c.SendStatus(503)
		}

		return c.Status(200).SendString("OK")
	})

}
