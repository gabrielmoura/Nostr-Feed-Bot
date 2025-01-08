package db

import (
	"Nostr-feed-bot/internal/db"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/goccy/go-json"
	"strings"
)

func GetRssToFeed() (db.FeedData, error) {
	mu.RLock()
	defer mu.RUnlock()
	events := make(db.FeedData)
	prefix := getFeedKey("")

	iter, _ := dbx.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefix),
	})
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var feed db.Feed
		if err := json.Unmarshal(iter.Value(), &feed); err != nil {
			return nil, fmt.Errorf("error unmarshalling feed: %w", err)
		}
		if feed.Url == "" || feed.Relay == "" || feed.PrivKey == "" || feed.PubKey == "" {
			continue
		}
		name := strings.ReplaceAll(string(iter.Key()), prefix, "")
		events[name] = &feed
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("error during iteration: %w", err)
	}
	return events, nil
}
func GetRssWithPublishedLinks() ([]*db.FeedResponse, error) {
	mu.RLock()
	defer mu.RUnlock()
	feeds := make(db.FeedData)

	prefix := getFeedKey("")

	iter, _ := dbx.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefix),
	})
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var feed db.Feed
		if err := json.Unmarshal(iter.Value(), &feed); err != nil {
			return nil, fmt.Errorf("error unmarshalling feed: %w", err)
		}
		if feed.Url == "" || feed.Relay == "" || feed.PrivKey == "" || feed.PubKey == "" {
			continue
		}
		name := strings.ReplaceAll(string(iter.Key()), prefix, "")
		feeds[name] = &feed
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("error during iteration: %w", err)
	}

	var feedsResponse []*db.FeedResponse
	for name, feed := range feeds {
		links, _ := getPublished(name)

		feedsResponse = append(feedsResponse, &db.FeedResponse{
			Links:  links.Links,
			Url:    feed.Url,
			Name:   feed.Name,
			Relay:  feed.Relay,
			PubKey: feed.PubKey,
		})
	}
	return feedsResponse, nil
}

func AddRssToFeed(feed *db.Feed) error {
	mu.Lock()
	defer mu.Unlock()

	value, err := json.Marshal(feed)
	if err != nil {
		return fmt.Errorf("error marshalling feed: %w", err)
	}

	if err := dbx.Set([]byte(getFeedKey(feed.Name)), value, pebble.Sync); err != nil {
		return fmt.Errorf("error setting data to db: %w", err)
	}
	return nil
}
func CheckIfFeedExists(feedName string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, err, _ := dbx.Get([]byte(getFeedKey(feedName)))
	return err == nil
}
