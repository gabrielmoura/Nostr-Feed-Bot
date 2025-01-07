package db

import "sync"

type FeedData map[string]*Feed
type Feed struct {
	FeedRequest   `json:"feed_request"`
	PublishedLink []string `json:"published_link"`
	sync.Mutex
}
type FeedRequest struct {
	Url     string `json:"url"`
	Name    string `json:"name"`
	PubKey  string `json:"pub_key"`
	PrivKey string `json:"priv_key"`
	Relay   string `json:"relay"`
}
