package server

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SevenTV/EventAPI/src/redis"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

func Health(app fiber.Router, connPtr *int32) {
	mtx := sync.Mutex{}

	app.Get("/health", func(c *fiber.Ctx) error {
		mtx.Lock()
		defer mtx.Unlock()

		c.Set("X-Active-Connections", fmt.Sprint(atomic.LoadInt32(connPtr)))

		redisCtx, cancel := context.WithTimeout(c.Context(), time.Second*10)
		defer cancel()
		if err := redis.Client.Ping(redisCtx).Err(); err != nil {
			logrus.WithError(err).Error("health, REDIS IS DOWN")
			return c.SendStatus(503)
		}

		return c.Status(200).SendString("OK")
	})

}
