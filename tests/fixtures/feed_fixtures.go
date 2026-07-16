package fixtures

import (
	"encoding/json"
	"time"

	"github.com/nathanap/news-feed-backend/services/utctime"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestFeed(userID string) db.Feed {
	return db.Feed{
		ID:         "01900000-0000-7000-8000-000000000030",
		Status:     true,
		Name:       "Metallica Feed",
		Keywords:   json.RawMessage(`["metallica","rock","metal","music","concert"]`),
		UserID:     userID,
		CreatedAt:  utctime.New(time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)),
		ModifiedAt: utctime.NullTime{},
		RemovedAt:  utctime.NullTime{},
	}
}

func NewTestFeedAlt(userID string) db.Feed {
	return db.Feed{
		ID:         "01900000-0000-7000-8000-000000000031",
		Status:     true,
		Name:       "Anime Feed",
		Keywords:   json.RawMessage(`["anime","naruto","cosplay","manga","japan"]`),
		UserID:     userID,
		CreatedAt:  utctime.New(time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)),
		ModifiedAt: utctime.NullTime{},
		RemovedAt:  utctime.NullTime{},
	}
}
