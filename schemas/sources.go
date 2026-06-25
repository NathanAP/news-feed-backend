package schemas

import "time"

type CreateSourceRequest struct {
	URL    string `json:"url"`
	URLRss string `json:"url_rss"`
}

type UpdateSourceRequest struct {
	URL    string `json:"url"`
	URLRss string `json:"url_rss"`
}

type SourceResponse struct {
	ID         string     `json:"id"`
	Status     bool       `json:"status"`
	URL        string     `json:"url"`
	URLRss     string     `json:"url_rss"`
	CreatedAt  time.Time  `json:"created_at"`
	ModifiedAt *time.Time `json:"modified_at,omitempty"`
}

type RSSDiscoveryResponse struct {
	Feeds []string `json:"feeds"`
}
