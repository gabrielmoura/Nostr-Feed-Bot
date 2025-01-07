package db

import (
	"Nostr-feed-bot/infra/util"
	"Nostr-feed-bot/internal/db"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/goccy/go-json"
	"github.com/nbd-wtf/go-nostr"
	"slices"
	"strings"
)

var (
	Data        db.FeedData
	dPrefix     = "feed_"
	eventPrefix = "event_" // event_ + rss_name +_+ eventID (hash of event)
	dbx         *pebble.DB
)

func init() {
	Data = make(db.FeedData)
}

func SetupDb() (*pebble.DB, error) {
	db, err := pebble.Open("pebble_data", &pebble.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	dbx = db
	return db, nil
}
func GetRssToFeed() (db.FeedData, error) {
	if util.CheckMapFeedEmpty(Data) {
		return Data, nil
	}

	iter, _ := dbx.NewIter(&pebble.IterOptions{
		LowerBound: []byte(dPrefix),
	})
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var feed db.Feed
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

func AddRssToFeed(feed *db.Feed) error {
	//Data[feed.Url] = feed
	value, err := json.Marshal(feed)
	if err != nil {
		return fmt.Errorf("error marshalling feed: %w", err)
	}

	if err := dbx.Set([]byte(dPrefix+feed.Url), value, pebble.Sync); err != nil {
		return fmt.Errorf("error setting data to db: %w", err)
	}
	return nil
}
func SaveEventToDb(feed *db.Feed, ev nostr.Event, id string) error {
	value, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("error marshalling event: %w", err)
	}

	key := fmt.Sprintf("%s%s_%s", eventPrefix, feed.Url, id)

	if err := dbx.Set([]byte(key), value, pebble.Sync); err != nil {
		return fmt.Errorf("error setting event to db: %w", err)
	}
	return nil
}
func GetAllEventsFromDb() ([]nostr.Event, error) {
	var events []nostr.Event
	iter, _ := dbx.NewIter(&pebble.IterOptions{
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
func GetAllEventsToPublish(feed *db.Feed) ([]nostr.Event, error) {
	var events []nostr.Event
	prefix := eventPrefix + feed.Url
	iter, _ := dbx.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefix),
		UpperBound: []byte(prefix + "_"),
	})
	defer iter.Close()

	// para todos os eventos que não estiver em published_link, publicar

	for iter.First(); iter.Valid(); iter.Next() {
		var ev nostr.Event
		if err := json.Unmarshal(iter.Value(), &ev); err != nil {
			return nil, fmt.Errorf("error unmarshalling event: %w", err)
		}

		url := strings.ReplaceAll(string(iter.Key()), prefix+"_", "")

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
func CheckIfEventExists(feed *db.Feed, id string) bool {
	key := fmt.Sprintf("%s%s_%s", eventPrefix, feed.Url, id)
	if _, err, _ := dbx.Get([]byte(key)); err != nil {
		return false
	}
	return true
}

func MarkEventAsPublished(feed *db.Feed, id string) error {
	feed.Lock()
	defer feed.Unlock()

	feed.PublishedLink = append(feed.PublishedLink, id)

	value, err := json.Marshal(feed)
	if err != nil {
		return fmt.Errorf("error marshalling feed: %w", err)
	}

	if err := dbx.Set([]byte(dPrefix+feed.Url), value, pebble.Sync); err != nil {
		return fmt.Errorf("error setting data to db: %w", err)
	}
	return nil
}
