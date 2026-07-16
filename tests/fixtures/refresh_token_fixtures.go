package fixtures

import (
	"time"

	"github.com/nathanap/news-feed-backend/services/utctime"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestRefreshToken(userID string) db.RefreshToken {
	return db.RefreshToken{
		ID:         "01900000-0000-7000-8000-000000000002",
		UserID:     userID,
		Status:     true,
		ExpiresAt:  utctime.New(time.Now().Add(30 * 24 * time.Hour)),
		CreatedAt:  utctime.New(time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)),
		ModifiedAt: utctime.NullTime{},
		RemovedAt:  utctime.NullTime{},
	}
}

func NewExpiredTestRefreshToken(userID string) db.RefreshToken {
	t := NewTestRefreshToken(userID)
	t.ExpiresAt = utctime.New(time.Now().Add(-time.Hour))
	return t
}
