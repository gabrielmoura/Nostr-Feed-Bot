package cron

import (
	"Nostr-feed-bot/infra/db"
	"fmt"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/gofiber/fiber/v2/log"
	"github.com/mmcdole/gofeed"
	"github.com/robfig/cron/v3"
)

var (
	feedParser  *gofeed.Parser
	mdConverter *converter.Converter
)

func init() {
	mdConverter = setupMdParser()
	feedParser = gofeed.NewParser()
	feedParser.UserAgent = "NostrFeedBot/0.1"
}
func setupMdParser() *converter.Converter {
	return converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(
				commonmark.WithStrongDelimiter("__"),
			),
		),
	)
}

func SetupCron() *cron.Cron {
	c := cron.New()

	c.AddFunc("*/2 * * * *", func() {
		feeds, err := db.GetRssToFeed()
		if err != nil {
			fmt.Println("Error getting feeds for cron: ", err)
			return
		}
		for _, feed := range feeds {
			processFeedItems(feed)
		}
	})

	c.AddFunc("* * * * *", func() {
		log.Info("Running cron job 1m")
		feeds, err := db.GetRssToFeed()
		if err != nil {
			log.Error("Error getting feeds for cron: ", err)
			return
		}
		for _, feed := range feeds {
			go PublishEvents(feed)
		}
	})

	return c
}
