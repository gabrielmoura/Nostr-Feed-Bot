package db

import (
	"Nostr-feed-bot/internal/db"
	"fmt"
	"github.com/cockroachdb/pebble"
	"github.com/goccy/go-json"
	"github.com/nbd-wtf/go-nostr"
	"strings"
)

func SaveEventToDb(feedName string, ev nostr.Event, itemUrl string) error {
	mu.Lock()
	defer mu.Unlock()
	value, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("error marshalling event: %w", err)
	}

	if err := dbx.Set([]byte(getEventKey(feedName, itemUrl)), value, pebble.Sync); err != nil {
		return fmt.Errorf("error setting event to db: %w", err)
	}
	return nil
}
func GetAllEventsFromDb() ([]nostr.Event, error) {
	mu.RLock()
	defer mu.RUnlock()
	var events []nostr.Event
	iter, _ := dbx.NewIter(&pebble.IterOptions{
		LowerBound: []byte(eventPrefix + "_"),
	})
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		if !strings.Contains(string(iter.Key()), eventPrefix) {
			continue
		}
		var ev nostr.Event
		if err := json.Unmarshal(iter.Value(), &ev); err != nil {
			return nil, fmt.Errorf("error unmarshalling event: %w", err)
		}
		if ok, _ := ev.CheckSignature(); !ok {
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
	mu.RLock()
	defer mu.RUnlock()
	var events []nostr.Event
	prefix := getEventKey(feed.Name, "")

	iter, _ := dbx.NewIter(&pebble.IterOptions{
		LowerBound: []byte(prefix),
	})
	defer iter.Close()

	// para todos os eventos que não estiver em published_link, publicar

	for iter.First(); iter.Valid(); iter.Next() {

		if !strings.Contains(string(iter.Key()), prefix) {
			continue
		}
		var ev nostr.Event
		if err := json.Unmarshal(iter.Value(), &ev); err != nil {
			return nil, fmt.Errorf("error unmarshalling event: %w", err)
		}
		if ok, _ := ev.CheckSignature(); !ok {
			continue
		}

		url := strings.ReplaceAll(string(iter.Key()), prefix, "")

		if CheckHasPublished(feed, url) {
			continue
		}

		events = append(events, ev)
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("error during iteration: %w", err)
	}
	return events, nil
}
func CheckIfEventExists(feed *db.Feed, itemUrl string) bool {
	mu.RLock()
	defer mu.RUnlock()
	if _, err, _ := dbx.Get([]byte(getEventKey(feed.Name, itemUrl))); err != nil {
		return false
	}
	return true
}
