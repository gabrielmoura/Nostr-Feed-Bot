package db

type FeedData map[string]*Feed
type Feed struct {
	FeedRequest `json:"feed_request"`
	//sync.RWMutex `json:"-"`
}
type Published struct {
	Links []string `json:"links"`
	Feed  string   `json:"feed"`
	//sync.RWMutex `json:"-"`
}
type FeedRequest struct {
	Url     string `json:"url"`
	Name    string `json:"name"`
	PubKey  string `json:"pub_key"`
	PrivKey string `json:"priv_key"`
	Relay   string `json:"relay"`
}
type FeedResponse struct {
	Url    string   `json:"url"`
	Name   string   `json:"name"`
	PubKey string   `json:"pub_key"`
	Relay  string   `json:"relay"`
	Links  []string `json:"links,omitempty"`
}
