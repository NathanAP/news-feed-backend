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

type DiscoveredArticleResponse struct {
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	URLOriginal string     `json:"url_original"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	SourceID    string     `json:"source_id"`
}

type SourceDiscoveryResponse struct {
	Articles []DiscoveredArticleResponse `json:"articles"`
}
