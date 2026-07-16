package fixtures

import (
	"database/sql"
	"encoding/json"
	"time"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestArticle() db.Article {
	return db.Article{
		ID:          "01900000-0000-7000-8000-000000000020",
		Status:      true,
		Title:       "Test Article",
		Content:     "# Test Article\n\nThis is a test article in markdown.",
		UrlOriginal: "https://example.com/news/test-article",
		Keywords:    json.RawMessage(`["metallica","rock","metal","music","concert"]`),
		SourceID:    NewTestSource().ID,
		CreatedAt:   time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
		ModifiedAt:  sql.NullTime{Valid: false},
		RemovedAt:   sql.NullTime{Valid: false},
	}
}

func NewTestArticleAlt() db.Article {
	return db.Article{
		ID:          "01900000-0000-7000-8000-000000000021",
		Status:      true,
		Title:       "Another Article",
		Content:     "# Another Article\n\nDifferent content here.",
		UrlOriginal: "https://other-site.com/news/another",
		Keywords:    json.RawMessage(`["anime","naruto","cosplay","manga","japan"]`),
		SourceID:    NewTestSource().ID,
		CreatedAt:   time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC),
		ModifiedAt:  sql.NullTime{Valid: false},
		RemovedAt:   sql.NullTime{Valid: false},
	}
}
