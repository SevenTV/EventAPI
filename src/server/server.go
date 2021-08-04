package server

import (
	"context"
	"net"
	"sync"

	"github.com/gofiber/fiber/v2"

	log "github.com/sirupsen/logrus"
)

func New(ctx context.Context, connType, connURI string) (*fiber.App, <-chan struct{}) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		StreamRequestBody:     true,
	})

	wg := sync.WaitGroup{}

	app.Use(Logger())
	app.Use(func(c *fiber.Ctx) error {
		wg.Add(1)
		c.SetUserContext(ctx)
		defer wg.Done()
		return c.Next()
	})

	Health(app)
	Testing(app)
	EventsV1(app)

	ln, err := net.Listen(connType, connURI)
	if err != nil {
		log.WithError(err).Fatal("failed to make listener")
	}

	go func() {
		if err := app.Listener(ln); err != nil {
			log.WithError(err).Fatal("failed to start server")
		}
	}()

	done := make(chan struct{})
	go func() {
		<-ctx.Done()
		wg.Wait()
		_ = ln.Close()
		close(done)
	}()

	return app, done
}
