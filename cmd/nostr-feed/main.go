package main

import (
	"Nostr-feed-bot/infra/cron"
	"Nostr-feed-bot/infra/db"
	"Nostr-feed-bot/infra/http"
	"fmt"
	"github.com/gofiber/fiber/v2/log"
)

func main() {
	dbx, err := db.SetupDb()
	defer dbx.Close()
	if err != nil {
		log.Fatal("Failed to setup database: ", err)
	}

	c := cron.SetupCron()
	go c.Start()

	app := http.SetupRoutes()
	if err := app.Listen(":3000"); err != nil {
		panic(fmt.Errorf("failed to start server: %w", err))
	}
}
