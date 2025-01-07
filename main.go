package main

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/cockroachdb/pebble"
	"github.com/goccy/go-json"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/mmcdole/gofeed"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/robfig/cron/v3"
)

type Feed struct {
	FeedRequest   `json:"feed_request"`
	PublishedLink []string   `json:"published_link"`
	mu            sync.Mutex `json:"-"`
}

type FeedRequest struct {
	Url     string `json:"url"`
	Name    string `json:"name"`
	PubKey  string `json:"pub_key"`
	PrivKey string `json:"priv_key"`
	Relay   string `json:"relay"`
}

type FeedData map[string]*Feed

var (
	db             *pebble.DB
	mdConverter    *converter.Converter
	snakeCaseRegex *regexp.Regexp
	feedParser     *gofeed.Parser
	Data           FeedData
	dPrefix        = "feed_"
	eventPrefix    = "event_" // event_ + rss_name +_+ eventID (hash of event)
)

func init() {
	mdConverter = setupMdParser()
	snakeCaseRegex = regexp.MustCompile("[^a-zA-Z0-9]+")
	feedParser = gofeed.NewParser()
	feedParser.UserAgent = "NostrFeedBot/0.1"
	Data = make(FeedData)
}

func main() {
	var err error
	db, err = pebble.Open("pebble_data", &pebble.Options{})
	if err != nil {
		panic(fmt.Errorf("failed to open database: %w", err))
	}
	defer db.Close()

	c := setupCron()
	go c.Start()

	app := setupRoutes()
	if err := app.Listen(":3000"); err != nil {
		panic(fmt.Errorf("failed to start server: %w", err))
	}
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

func DecodeKey(key string) string {
	if _, decoded, _ := nip19.Decode(key); decoded != nil {
		return decoded.(string)
	}
	return ""
}

func setupRoutes() *fiber.App {
	app := fiber.New(fiber.Config{
		JSONEncoder: json.Marshal,
		JSONDecoder: json.Unmarshal,
		AppName:     "NostrFeed",
	})
	app.Use(logger.New())
	app.Use(etag.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	app.Post("/rss", addRssHandler)
	app.Get("/rss", getRssHandler)

	app.Get("/events", listEventsHandler)

	return app
}

func listEventsHandler(c *fiber.Ctx) error {
	events, err := GetAllEventsFromDb()
	if err != nil {
		return fiberError(c, fiber.StatusInternalServerError, "error getting events", err)
	}
	return c.JSON(events)
}

func addRssHandler(c *fiber.Ctx) error {
	req := new(FeedRequest)
	if err := c.BodyParser(req); err != nil {
		return fiberError(c, fiber.StatusBadRequest, "invalid request", err)
	}

	if req.Url == "" || req.PubKey == "" || req.PrivKey == "" || req.Relay == "" {
		return fiberError(c, fiber.StatusBadRequest, "missing required fields", nil)
	}
	if Data[req.Url] != nil {
		return fiberError(c, fiber.StatusBadRequest, "feed already exists", nil)
	}

	feed := &Feed{
		FeedRequest: *req,
	}

	if err := AddRssToFeed(feed); err != nil {
		return fiberError(c, fiber.StatusInternalServerError, "error adding feed", err)
	}

	return c.JSON(fiber.Map{"success": true, "message": "rss feed added"})
}

func getRssHandler(c *fiber.Ctx) error {
	feeds, err := GetRssToFeed()
	if err != nil {
		return fiberError(c, fiber.StatusInternalServerError, "error getting feeds", err)
	}
	return c.JSON(feeds)
}

func fiberError(c *fiber.Ctx, status int, message string, err error) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"message": message,
		"error":   err.Error(),
	})
}
func CheckMapFeedEmpty(m FeedData) bool {
	if m != nil && len(m) > 0 {
		return true
	}
	return false
}

func GetRssToFeed() (FeedData, error) {
	if CheckMapFeedEmpty(Data) {
		return Data, nil
	}

	iter, _ := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(dPrefix),
	})
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var feed Feed
		if err := json.Unmarshal(iter.Value(), &feed); err != nil {
			return nil, fmt.Errorf("error unmarshalling feed: %w", err)
		}
		Data[string(iter.Key())] = &feed
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("error during iteration: %w", err)
	}
	return Data, nil
}

func AddRssToFeed(feed *Feed) error {
	//Data[feed.Url] = feed
	value, err := json.Marshal(feed)
	if err != nil {
		return fmt.Errorf("error marshalling feed: %w", err)
	}

	if err := db.Set([]byte(dPrefix+feed.Url), value, pebble.Sync); err != nil {
		return fmt.Errorf("error setting data to db: %w", err)
	}
	return nil
}
func SaveEventToDb(feed *Feed, ev nostr.Event, id string) error {
	value, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("error marshalling event: %w", err)
	}

	key := fmt.Sprintf("%s%s_%s", eventPrefix, feed.Url, id)

	if err := db.Set([]byte(key), value, pebble.Sync); err != nil {
		return fmt.Errorf("error setting event to db: %w", err)
	}
	return nil
}
func GetAllEventsFromDb() ([]nostr.Event, error) {
	var events []nostr.Event
	iter, _ := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(eventPrefix),
	})
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var ev nostr.Event
		if err := json.Unmarshal(iter.Value(), &ev); err != nil {
			return nil, fmt.Errorf("error unmarshalling event: %w", err)
		}
		if ev.Kind == 0 {
			continue
		}
		events = append(events, ev)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("error during iteration: %w", err)
	}
	return events, nil
}
func GetAllEventsToPublish(feed *Feed) ([]nostr.Event, error) {
	var events []nostr.Event
	iter, _ := db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(eventPrefix + feed.Url + "_"),
	})
	defer iter.Close()

	// para todos os eventos que não estiver em published_link, publicar

	for iter.First(); iter.Valid(); iter.Next() {
		var ev nostr.Event
		if err := json.Unmarshal(iter.Value(), &ev); err != nil {
			return nil, fmt.Errorf("error unmarshalling event: %w", err)
		}

		url := strings.ReplaceAll(string(iter.Key()), eventPrefix+feed.Url+"_", "")

		if slices.Contains(feed.PublishedLink, url) {
			continue
		}
		if ev.Kind == 0 {
			continue
		}
		events = append(events, ev)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("error during iteration: %w", err)
	}
	return events, nil
}

func CheckIfEventExists(feed *Feed, id string) bool {
	key := fmt.Sprintf("%s%s_%s", eventPrefix, feed.Url, id)
	if _, err, _ := db.Get([]byte(key)); err != nil {
		return false
	}
	return true
}

func setupCron() *cron.Cron {
	c := cron.New()

	c.AddFunc("*/2 * * * *", func() {
		feeds, err := GetRssToFeed()
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
		feeds, err := GetRssToFeed()
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

func processFeedItems(feed *Feed) {
	items, err := parseUrl(feed.Url)
	if err != nil {
		log.Error("Error parsing feed: ", feed.Url, err)
		return
	}

	for _, item := range items {
		ProcessFeedItem(feed, item)
	}
}

func parseUrl(url string) ([]*gofeed.Item, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	feed, err := feedParser.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}
	return feed.Items, nil
}

func ProcessFeedItem(feed *Feed, item *gofeed.Item) {
	if item.Link == "" {
		return
	}

	feed.mu.Lock()
	if slices.Contains(feed.PublishedLink, item.Link) {
		feed.mu.Unlock()
		return
	}
	if !CheckIfEventExists(feed, item.Link) {
		feed.mu.Unlock()
		return
	}
	feed.PublishedLink = append(feed.PublishedLink, item.Link)
	feed.mu.Unlock()

	markdown, err := mdConverter.ConvertString(TernaryString(item.Content, item.Description))
	if err != nil {
		log.Error("Error converting to markdown: ", err)
		return
	}

	tags := createTags(item)
	ev := nostr.Event{
		Kind:      nostr.KindArticle,
		CreatedAt: nostr.Now(),
		PubKey:    DecodeKey(feed.PubKey),
		Content:   markdown,
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

	log.Debug("Event: ", ev)
	err = SaveEventToDb(feed, ev, item.Link)
	if err != nil {
		log.Error("Error saving event to db: ", err)
	}
}

func createTags(item *gofeed.Item) []nostr.Tag {
	tags := []nostr.Tag{
		{"title", item.Title, ""},
		{"proxy", item.Link, "activitypub"},
		{"d", ToSnakeCase(item.Title), ""},
	}

	for _, category := range item.Categories {
		tags = append(tags, nostr.Tag{"t", category, ""})
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

func PublishEvents(feed *Feed) {
	ctx := context.Background()
	rs, err := nostr.RelayConnect(ctx, feed.Relay)
	if err != nil {
		log.Error("Failed to connect to relay: ", err)
		return
	}
	defer rs.Close()

	// para todos os eventos que não estiver em published_link, publicar

	events, err := GetAllEventsToPublish(feed)
	if err != nil {
		log.Error("Error getting events to publish: ", err)
		return
	}

	for _, ev := range events {
		time.Sleep(1 * time.Second)
		if err := rs.Publish(ctx, ev); err != nil {
			log.Error("Failed to publish message: ", err)
			continue
		}
		err := MarkEventAsPublished(feed, ev.Tags.GetFirst([]string{"proxy"}).Value())
		if err != nil {
			log.Error("Failed to mark event as published: ", err)
			continue
		}
	}

}
func MarkEventAsPublished(feed *Feed, id string) error {
	feed.mu.Lock()
	defer feed.mu.Unlock()

	feed.PublishedLink = append(feed.PublishedLink, id)

	value, err := json.Marshal(feed)
	if err != nil {
		return fmt.Errorf("error marshalling feed: %w", err)
	}

	if err := db.Set([]byte(dPrefix+feed.Url), value, pebble.Sync); err != nil {
		return fmt.Errorf("error setting data to db: %w", err)
	}
	return nil
}

func ToSnakeCase(str string) string {
	str = snakeCaseRegex.ReplaceAllString(str, "_")
	return strings.ToLower(strings.TrimSpace(str))
}

func TernaryString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
