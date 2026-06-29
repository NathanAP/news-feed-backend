package schemas

import "time"

const (
	ArticleKeywordsMin = 5
	ArticleKeywordsMax = 20
)

type CreateArticleRequest struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	URLOriginal string   `json:"url_original"`
	Keywords    []string `json:"keywords"`
	SourceID    string   `json:"source_id"`
}

// UpdateArticleRequest intentionally omits source_id: the article's source is immutable.
type UpdateArticleRequest struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	URLOriginal string   `json:"url_original"`
	Keywords    []string `json:"keywords"`
}

type ArticleResponse struct {
	ID          string     `json:"id"`
	Status      bool       `json:"status"`
	Title       string     `json:"title"`
	Content     string     `json:"content"`
	URLOriginal string     `json:"url_original"`
	Keywords    []string   `json:"keywords"`
	SourceID    string     `json:"source_id"`
	CreatedAt   time.Time  `json:"created_at"`
	ModifiedAt  *time.Time `json:"modified_at,omitempty"`
	// IsRead is null when the article is not in any of the requesting user's feeds,
	// false when it is in at least one feed and unread, true when all are read.
	IsRead *bool `json:"is_read"`
}
