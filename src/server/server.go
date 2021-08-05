package server

import (
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/SevenTV/EventAPI/src/configure"
	"github.com/SevenTV/EventAPI/src/utils"
	"github.com/gofiber/fiber/v2"

	log "github.com/sirupsen/logrus"
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
		c.Set("X-Node-ID", configure.Config.GetString("node_id"))
		return c.Next()
	})

	Health(app, conns)
	Testing(app)
	public := app.Group("/public")
	startCb := func() {
		wg.Add(1)
		atomic.AddInt32(conns, 1)
	}
	doneCb := func() {
		atomic.AddInt32(conns, -1)
		wg.Done()
	}
	EventsV1(public, startCb, doneCb)

	app.Use(func(c *fiber.Ctx) error {
		return c.SendStatus(404)
	})

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
