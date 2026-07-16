package fixtures

import (
	"time"

	"github.com/nathanap/news-feed-backend/services/utctime"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestArticleFeed(articleID, feedID string) db.ArticlesFeed {
	return db.ArticlesFeed{
		ID:         "01900000-0000-7000-8000-000000000050",
		ArticleID:  articleID,
		FeedID:     feedID,
		IsRead:     false,
		CreatedAt:  utctime.New(time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)),
		ModifiedAt: utctime.NullTime{},
	}
}
