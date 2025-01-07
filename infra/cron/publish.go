package cron

import (
	db2 "Nostr-feed-bot/infra/db"
	"Nostr-feed-bot/internal/db"
	"context"
	"github.com/gofiber/fiber/v2/log"
	"github.com/nbd-wtf/go-nostr"
	"time"
)

func PublishEvents(feed *db.Feed) {
	ctx := context.Background()
	rs, err := nostr.RelayConnect(ctx, feed.Relay)
	if err != nil {
		log.Error("Failed to connect to relay: ", err)
		return
	}
	defer rs.Close()

	// para todos os eventos que não estiver em published_link, publicar

	events, err := db2.GetAllEventsToPublish(feed)
	if err != nil {
		log.Error("Error getting events to publish: ", err)
		return
	}

	for _, ev := range events {
		time.Sleep(1 * time.Second)
		if err := rs.Publish(ctx, ev); err != nil {
			log.Error("Failed to publish message: ", err)
			continue
		} else {
			err := db2.MarkEventAsPublished(feed, ev.Tags.GetFirst([]string{"proxy"}).Value())
			if err != nil {
				log.Error("Failed to mark event as published: ", err)
				continue
			}
		}
	}

}
