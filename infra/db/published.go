package db

import (
	"Nostr-feed-bot/internal/db"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/goccy/go-json"
	"slices"
	"strings"
)

func getPublished(feedName string) (db.Published, error) {
	mu.RLock()
	defer mu.RUnlock()
	prefix := getPublishedKey(feedName)
	iter, _ := dbx.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefix),
	})
	defer iter.Close()
	var published db.Published
	for iter.First(); iter.Valid(); iter.Next() {
		if !strings.Contains(string(iter.Key()), prefix) {
			continue
		}
		if err := json.Unmarshal(iter.Value(), &published); err != nil {
			return published, fmt.Errorf("error unmarshalling published: %w", err)
		}
		if published.Feed != feedName {
			continue
		}
	}
	if err := iter.Error(); err != nil {
		return published, fmt.Errorf("error during iteration: %w", err)
	}
	return published, nil
}
func setPublished(feedID string, published db.Published) error {
	mu.Lock()
	defer mu.Unlock()
	value, err := json.Marshal(published)
	if err != nil {
		return fmt.Errorf("error marshalling published: %w", err)
	}
	if err := dbx.Set([]byte(getPublishedKey(feedID)), value, pebble.Sync); err != nil {
		return fmt.Errorf("error setting published to db: %w", err)
	}
	return nil
}

func CheckHasPublished(feed *db.Feed, itemUrl string) bool {
	published, err := getPublished(feed.Name)
	if err != nil {
		return false
	}
	return slices.Contains(published.Links, itemUrl)
}

func MarkEventAsPublished(feed *db.Feed, itemUrl string) error {
	published, err := getPublished(feed.Name)
	if err != nil {
		return fmt.Errorf("error getting published: %w", err)
	}
	if len(published.Links) == 0 {
		published.Links = []string{itemUrl}
		published.Feed = feed.Url
	} else {
		published.Links = append(published.Links, itemUrl)
	}

	if err := setPublished(feed.Name, published); err != nil {
		return fmt.Errorf("error setting published to db: %w", err)
	}

	return nil
}
