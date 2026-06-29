package fixtures

import (
	"database/sql"
	"time"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestArticleFeed(articleID, feedID string) db.ArticleFeed {
	return db.ArticleFeed{
		ID:         "01900000-0000-7000-8000-000000000050",
		ArticleID:  articleID,
		FeedID:     feedID,
		IsRead:     0,
		CreatedAt:  time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC),
		ModifiedAt: sql.NullTime{Valid: false},
	}
}
