package schemas

import "time"

const (
	FeedKeywordsMin   = 5
	FeedKeywordsMax   = 20
	FeedNameMaxLength = 120
	FeedMaxPerUser    = 5
)

type CreateFeedRequest struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}

// UpdateFeedRequest intentionally omits user_id: a feed's owner is immutable.
type UpdateFeedRequest struct {
	Name     string   `json:"name"`
	Keywords []string `json:"keywords"`
}

type FeedResponse struct {
	ID         string     `json:"id"`
	Status     bool       `json:"status"`
	Name       string     `json:"name"`
	Keywords   []string   `json:"keywords"`
	UserID     string     `json:"user_id"`
	CreatedAt  time.Time  `json:"created_at"`
	ModifiedAt *time.Time `json:"modified_at,omitempty"`
}
