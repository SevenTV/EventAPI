package server

import "github.com/gofiber/fiber/v2"

func Testing(app fiber.Router) {
	testing := app.Group("/testing")
	testing.Get("/panic", func(c *fiber.Ctx) error {
		panic("testing panic")
	})
}
