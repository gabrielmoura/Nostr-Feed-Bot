package cron

import (
	db2 "Nostr-feed-bot/infra/db"
	"Nostr-feed-bot/infra/util"
	"Nostr-feed-bot/internal/db"
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2/log"
	"github.com/mmcdole/gofeed"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"strconv"
	"strings"
	"time"
)

func parseUrl(url string) ([]*gofeed.Item, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	feed, err := feedParser.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}
	return feed.Items, nil
}
func processFeedItems(feed *db.Feed) {
	items, err := parseUrl(feed.Url)
	if err != nil {
		log.Error("Error parsing feed: ", feed.Url, err)
		return
	}

	for _, item := range items {
		ProcessFeedItem(feed, item)
	}
}
func createTags(item *gofeed.Item) []nostr.Tag {
	tags := []nostr.Tag{
		{"title", item.Title, ""},
		{"proxy", item.Link, "activitypub"},
		{"d", util.ToSnakeCase(item.Title), ""},
	}

	for _, category := range item.Categories {
		if category != "" {
			tags = append(tags, nostr.Tag{"t", util.ToSnakeCase(category), ""})
		}
	}

	if summary := item.Custom["summary"]; summary != "" {
		summaryMd, _ := mdConverter.ConvertString(summary)
		tags = append(tags, nostr.Tag{"summary", summaryMd, ""})
	}

	if item.Image != nil {
		tags = append(tags, nostr.Tag{"image", item.Image.URL, ""})
	}

	if authors := item.Authors; len(authors) > 0 {
		tags = append(tags, nostr.Tag{"author", authors[0].Name, ""})
	}

	if published := item.PublishedParsed; published != nil {
		tags = append(tags, nostr.Tag{"published_at", strconv.Itoa(int(published.Unix())), ""})
	}

	return tags
}
func createHashtag(item *gofeed.Item) string {
	str := strings.Builder{}
	for _, category := range item.Categories {
		str.WriteString("#")
		str.WriteString(util.ToSnakeCase(category))
		str.WriteString(" ")
	}
	return str.String()
}
func DecodeKey(key string) string {
	if _, decoded, _ := nip19.Decode(key); decoded != nil {
		return decoded.(string)
	}
	return ""
}

func ProcessFeedItem(feed *db.Feed, item *gofeed.Item) {
	guid := util.TernaryString(item.Link, item.GUID)
	if guid == "" {
		return
	}

	if db2.CheckHasPublished(feed, item.Link) {

		return
	}
	if !db2.CheckIfEventExists(feed, item.Link) {

		return
	}

	markdown, err := mdConverter.ConvertString(util.TernaryString(item.Content, item.Description))
	if err != nil {
		log.Error("Error converting to markdown: ", err)
		return
	}

	tags := createTags(item)
	hashTags := createHashtag(item)
	ev := nostr.Event{
		Kind:      nostr.KindTextNote,
		CreatedAt: nostr.Now(),
		PubKey:    DecodeKey(feed.PubKey),
		Content:   markdown + "\n\n" + hashTags,
		Tags:      tags,
	}

	if err := ev.Sign(DecodeKey(feed.PrivKey)); err != nil {
		log.Error("Error signing event: ", err)
		return
	}
	if ok := ev.CheckID(); !ok {
		log.Error("Error verifying event: ", err)
		return
	}

	err = db2.SaveEventToDb(feed.Name, ev, guid)
	if err != nil {
		log.Error("Error saving event to db: ", err)
	}
}
