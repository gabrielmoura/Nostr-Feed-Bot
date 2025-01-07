package http

import (
	"Nostr-feed-bot/infra/db"
	db2 "Nostr-feed-bot/internal/db"
	"github.com/gofiber/fiber/v2"
)

func listEventsHandler(c *fiber.Ctx) error {
	events, err := db.GetAllEventsFromDb()
	if err != nil {
		return fiberError(c, fiber.StatusInternalServerError, "error getting events", err)
	}
	return c.JSON(events)
}

func addRssHandler(c *fiber.Ctx) error {
	req := new(db2.FeedRequest)
	if err := c.BodyParser(req); err != nil {
		return fiberError(c, fiber.StatusBadRequest, "invalid request", err)
	}

	if req.Url == "" || req.PubKey == "" || req.PrivKey == "" || req.Relay == "" {
		return fiberError(c, fiber.StatusBadRequest, "missing required fields", nil)
	}
	if db.Data[req.Url] != nil {
		return fiberError(c, fiber.StatusBadRequest, "feed already exists", nil)
	}

	feed := &db2.Feed{
		FeedRequest: *req,
	}

	if err := db.AddRssToFeed(feed); err != nil {
		return fiberError(c, fiber.StatusInternalServerError, "error adding feed", err)
	}

	return c.JSON(fiber.Map{"success": true, "message": "rss feed added"})
}

func getRssHandler(c *fiber.Ctx) error {
	feeds, err := db.GetRssToFeed()
	if err != nil {
		return fiberError(c, fiber.StatusInternalServerError, "error getting feeds", err)
	}
	return c.JSON(feeds)
}
