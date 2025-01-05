package main

import (
	"context"
	"fmt"
	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/mmcdole/gofeed"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/robfig/cron/v3"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Feed struct {
	FeedRequest
	ToPublish     chan nostr.Event
	PublishedLink []string
	mu            sync.Mutex
}

type FeedRequest struct {
	Url     string `json:"url"`
	PubKey  string `json:"pub_key"`
	PrivKey string `json:"priv_key"`
	Relay   string `json:"relay"`
}

var (
	Data           = make(map[string]*Feed)
	dataMu         sync.RWMutex
	mdConverter    *converter.Converter
	snakeCaseRegex *regexp.Regexp
	feedParser     *gofeed.Parser
)

func init() {
	mdConverter = setupMdParser()
	snakeCaseRegex = regexp.MustCompile("[^a-zA-Z0-9]+")
	feedParser = gofeed.NewParser()
	feedParser.UserAgent = "NostrFeedBot/0.1"
}

func main() {
	c := setupCron()
	go c.Start()

	app := setupRoutes()
	log.Fatal(app.Listen(":3000"))
}

func setupMdParser() *converter.Converter {
	conv := converter.NewConverter(
		converter.WithPlugins(
			base.NewBasePlugin(),
			commonmark.NewCommonmarkPlugin(
				commonmark.WithStrongDelimiter("__"),
			),
		),
	)
	return conv
}

func DecodeKey(key string) string {
	if _, decoded, _ := nip19.Decode(key); decoded != nil {
		return decoded.(string)
	}
	return ""
}

func setupRoutes() *fiber.App {
	app := fiber.New()
	app.Use(logger.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})
	app.Post("/rss", func(c *fiber.Ctx) error {
		req := new(FeedRequest)
		if err := c.BodyParser(req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error":   "invalid request",
				"message": err.Error(),
			})
		}

		feed := &Feed{
			FeedRequest: FeedRequest{
				Url:     req.Url,
				PubKey:  DecodeKey(req.PubKey),
				PrivKey: DecodeKey(req.PrivKey),
				Relay:   req.Relay,
			},
			ToPublish: make(chan nostr.Event, 100), // Canal com buffer para evitar bloqueios
		}
		AddRssToFeed(feed)
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"message": "rss feed added",
		})
	})
	app.Get("/rss", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(GetRssToFeed())
	})
	return app
}

func GetRssToFeed() map[string]*Feed {
	dataMu.RLock()
	defer dataMu.RUnlock()
	return Data
}

func AddRssToFeed(feed *Feed) {
	dataMu.Lock()
	defer dataMu.Unlock()
	Data[feed.Url] = feed
}

// ProcessFeedItem processes a feed item and sends it to the relay
func ProcessFeedItem(feed *Feed, item *gofeed.Item) {
	if item.Link == "" {
		return
	}

	feed.mu.Lock()
	if slices.Contains(feed.PublishedLink, item.Link) {
		feed.mu.Unlock()
		return
	}
	feed.mu.Unlock()

	markdown, err := mdConverter.ConvertString(TernaryString(item.Content, item.Description))
	if err != nil {
		log.Error("Error converting to markdown", err)
		return
	}

	t := make([]nostr.Tag, 0)
	t = append(t, nostr.Tag{"title", item.Title, ""})
	t = append(t, nostr.Tag{"proxy", item.Link, "activitypub"})

	t = append(t, nostr.Tag{"d", ToSnakeCase(item.Title), ""})

	for _, category := range item.Categories {
		t = append(t, nostr.Tag{"t", category, ""})
	}
	if item.Custom["summary"] != "" {
		summary, err := mdConverter.ConvertString(item.Custom["summary"])
		if err != nil {
			log.Error("Error converting summary to markdown", err)
			return
		}
		t = append(t, nostr.Tag{"summary", summary, ""})
	}
	if item.Image != nil {
		t = append(t, nostr.Tag{"image", item.Image.URL, ""})
	}

	if item.Authors != nil {
		t = append(t, nostr.Tag{"author", item.Authors[0].Name, ""})
	}

	if item.PublishedParsed != nil {
		t = append(t, nostr.Tag{"published_at", strconv.Itoa(int(item.PublishedParsed.Unix())), ""})
	}

	ev := nostr.Event{
		Kind:      nostr.KindArticle,
		CreatedAt: nostr.Now(),
		PubKey:    feed.PubKey,
		Content:   markdown,
		Tags:      t,
	}

	err = ev.Sign(feed.PrivKey)
	if err != nil {
		log.Error("Error signing event", err)
		return
	}

	log.Debug(ev.String())
	feed.ToPublish <- ev

}

// PublishEvents publishes events to the relay
func PublishEvents(feed *Feed) {
	ctx := context.Background()
	rs, err := nostr.RelayConnect(ctx, feed.Relay)
	if err != nil {
		log.Error("failed to connect to relay", err)
		return
	}
	defer rs.Close()

	for ev := range feed.ToPublish {
		time.Sleep(1 * time.Second)
		err := rs.Publish(ctx, ev)
		if err != nil {
			log.Error("failed to publish message", err)
			continue
		}
		feed.mu.Lock()
		feed.PublishedLink = append(feed.PublishedLink, ev.Tags.GetFirst([]string{"proxy"}).Value())
		feed.mu.Unlock()
	}
	log.Info("PublishEvents goroutine finished for: ", feed.Url)
}

// setupCron sets up the cron job
func setupCron() *cron.Cron {
	c := cron.New()
	c.AddFunc("*/2 * * * *", func() {
		dataMu.RLock()
		feeds := Data
		dataMu.RUnlock()
		for _, feed := range feeds {
			items, err := parseUrl(feed.Url)
			if err != nil {
				log.Error("Error parsing feed: ", feed.Url, err)
				continue
			}
			log.Info("Processing feed: ", feed.Url, "Items:", len(items))
			for _, item := range items {
				go ProcessFeedItem(feed, item)
			}
		}
	})
	c.AddFunc("* * * * *", func() {
		dataMu.RLock()
		feeds := Data
		dataMu.RUnlock()
		for _, feed := range feeds {
			go PublishEvents(feed) // Inicia a goroutine de publicação
		}
	})
	return c
}

// parseUrl parses a URL and returns the feed items
func parseUrl(url string) ([]*gofeed.Item, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	feed, err := feedParser.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %w", err)
	}
	return feed.Items, nil
}

// ToSnakeCase converts a string to snake_case
func ToSnakeCase(str string) string {
	str = snakeCaseRegex.ReplaceAllString(str, "_")
	str = strings.TrimSpace(str)
	return strings.ToLower(str)
}

// TernaryString returns the first string if it is not empty, otherwise it returns the second string
func TernaryString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
