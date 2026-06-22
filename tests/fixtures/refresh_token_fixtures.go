package fixtures

import (
	"database/sql"
	"time"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestRefreshToken(userID string) db.RefreshToken {
	return db.RefreshToken{
		ID:         "01900000-0000-7000-8000-000000000002",
		UserID:     userID,
		Status:     1,
		ExpiresAt:  time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:  time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC),
		ModifiedAt: sql.NullTime{Valid: false},
		RemovedAt:  sql.NullTime{Valid: false},
	}
}

func NewExpiredTestRefreshToken(userID string) db.RefreshToken {
	t := NewTestRefreshToken(userID)
	t.ExpiresAt = time.Now().Add(-time.Hour)
	return t
}
