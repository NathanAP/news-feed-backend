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
}

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
	CreatedAt   time.Time  `json:"created_at"`
	ModifiedAt  *time.Time `json:"modified_at,omitempty"`
}
