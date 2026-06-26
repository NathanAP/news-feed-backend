package fixtures

import (
	"database/sql"
	"time"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestArticle() db.Article {
	return db.Article{
		ID:          "01900000-0000-7000-8000-000000000020",
		Status:      1,
		Title:       "Test Article",
		Content:     "# Test Article\n\nThis is a test article in markdown.",
		UrlOriginal: "https://example.com/news/test-article",
		Keywords:    `["metallica","rock","metal","music","concert"]`,
		CreatedAt:   time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
		ModifiedAt:  sql.NullTime{Valid: false},
		RemovedAt:   sql.NullTime{Valid: false},
	}
}

func NewTestArticleAlt() db.Article {
	return db.Article{
		ID:          "01900000-0000-7000-8000-000000000021",
		Status:      1,
		Title:       "Another Article",
		Content:     "# Another Article\n\nDifferent content here.",
		UrlOriginal: "https://other-site.com/news/another",
		Keywords:    `["anime","naruto","cosplay","manga","japan"]`,
		CreatedAt:   time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
		ModifiedAt:  sql.NullTime{Valid: false},
		RemovedAt:   sql.NullTime{Valid: false},
	}
}

// ValidArticleKeywords returns a keyword slice satisfying the 5-20 rule for request payloads.
func ValidArticleKeywords() []string {
	return []string{"metallica", "rock", "metal", "music", "concert"}
}
