package http

import (
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

func SetupRoutes() *fiber.App {
	log.SetLevel(log.LevelInfo)
	app := fiber.New(fiber.Config{
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
		AppName:     "NostrFeed",
	})
	app.Use(logger.New())
	app.Use(etag.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("NostrFeed")
	})

	app.Post("/rss", addRssHandler)
	app.Get("/rss", getRssHandler)

	app.Get("/events", listEventsHandler)

	return app
}
