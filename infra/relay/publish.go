package relay

import (
	"Nostr-feed-bot/infra/db"
	db2 "Nostr-feed-bot/internal/db"
	"context"
	"github.com/gofiber/fiber/v2/log"
	"github.com/nbd-wtf/go-nostr"
	"sync"
	"time"
)

var wg sync.WaitGroup

func InitPublisher() {
	for {
		feeds, err := db.GetRssToFeed()
		if len(feeds) == 0 {
			log.Debug("No feeds to publish,waiting 10 seconds")
			time.Sleep(30 * time.Second)
			continue
		}

		wg.Add(len(feeds))
		if err != nil {
			log.Error("Error getting feeds for cron: ", err)
			return
		}
		for _, feed := range feeds {
			go doPublish(feed)
		}
		wg.Wait()
		log.Debug("Finished publishing events, waiting 10 seconds to publish again")
		time.Sleep(10 * time.Second)
	}
}
func doPublish(feed *db2.Feed) {
	ctx := context.Background()
	rs, err := nostr.RelayConnect(ctx, feed.Relay)
	if err != nil {
		log.Error("Failed to connect to relay: ", err)
		return
	}
	defer func() {
		rs.Close()
		wg.Done()
	}()

	// para todos os eventos que não estiver em published_link, publicar

	events, err := db.GetAllEventsToPublish(feed)
	if err != nil {
		log.Error("Error getting events to publish: ", err)
		return
	}
	log.Debug("Number of events to publish: ", len(events))
	for _, ev := range events {
		time.Sleep(1 * time.Second)
		if err := rs.Publish(ctx, ev); err != nil {
			log.Error("Failed to publish message: ", err)
			continue
		} else {
			err := db.MarkEventAsPublished(feed, ev.Tags.GetFirst([]string{"proxy"}).Value())
			if err != nil {
				log.Error("Failed to mark event as published: ", err)
				continue
			}
		}
	}
}
