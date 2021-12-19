package server

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/SevenTV/EventAPI/src/configure"
	"github.com/SevenTV/EventAPI/src/utils"
	"github.com/gofiber/fiber/v2"

	"github.com/sirupsen/logrus"
)

func New(ctx context.Context, connType, connURI string) (*fiber.App, <-chan struct{}) {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		StreamRequestBody:     true,
	})

	wg := sync.WaitGroup{}

	conns := utils.Int32Pointer(0)

	app.Use(Logger())

	app.Use(func(c *fiber.Ctx) error {
		c.SetUserContext(ctx)

		c.Set("X-Node-Name", configure.NodeName)
		c.Set("X-Pod-Name", configure.PodName)
		c.Set("X-Pod-Internal-Address", configure.PodIP)

		// FFZ AP fix for bad url
		if c.Path() == "/public/v1//channel-emotes" {
			c.Path("/public/v1/channel-emotes")
		}

		return c.Next()
	})

	Health(app, conns)
	Testing(app)

	EventsV1(app.Group("/public"), func() {
		wg.Add(1)
		atomic.AddInt32(conns, 1)
	}, func() {
		atomic.AddInt32(conns, -1)
		wg.Done()
	})

	app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(404)
	})

	ln, err := net.Listen(connType, connURI)
	if err != nil {
		logrus.WithError(err).Fatal("failed to make listener")
	}

	go func() {
		if err := app.Listener(ln); err != nil {
			logrus.WithError(err).Fatal("failed to start server")
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
