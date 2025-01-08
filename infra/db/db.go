package db

import (
	"fmt"
	"github.com/cockroachdb/pebble"
	"sync"
)

var (
	dbx *pebble.DB
	mu  sync.RWMutex
)

const (
	feedPrefix      = "feed"
	eventPrefix     = "event"
	publishedPrefix = "published"
)

func getFeedKey(feedName string) string {
	return fmt.Sprintf("%s_%s", feedPrefix, feedName)
}
func getEventKey(feedName string, urlItem string) string {
	return fmt.Sprintf("%s_%s_%s", eventPrefix, feedName, urlItem)
}
func getPublishedKey(feedName string) string {
	return fmt.Sprintf("%s_%s", publishedPrefix, feedName)
}

func SetupDb() (*pebble.DB, error) {
	d, err := pebble.Open("pebble_data", &pebble.Options{})
	d.Flush()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	dbx = d
	return d, nil
}
func FlushDb() (*pebble.Metrics, error) {
	mu.Lock()
	defer mu.Unlock()

	err := dbx.Flush()
	if err != nil {
		return nil, fmt.Errorf("failed to flush database: %w", err)
	}
	return dbx.Metrics(), nil
}
