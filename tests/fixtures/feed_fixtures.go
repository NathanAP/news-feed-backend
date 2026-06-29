package fixtures

import (
	"database/sql"
	"time"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestFeed(userID string) db.Feed {
	return db.Feed{
		ID:         "01900000-0000-7000-8000-000000000030",
		Status:     1,
		Name:       "Metallica Feed",
		Keywords:   `["metallica","rock","metal","music","concert"]`,
		UserID:     userID,
		CreatedAt:  time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
		ModifiedAt: sql.NullTime{Valid: false},
		RemovedAt:  sql.NullTime{Valid: false},
	}
}

func NewTestFeedAlt(userID string) db.Feed {
	return db.Feed{
		ID:         "01900000-0000-7000-8000-000000000031",
		Status:     1,
		Name:       "Anime Feed",
		Keywords:   `["anime","naruto","cosplay","manga","japan"]`,
		UserID:     userID,
		CreatedAt:  time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC),
		ModifiedAt: sql.NullTime{Valid: false},
		RemovedAt:  sql.NullTime{Valid: false},
	}
}
