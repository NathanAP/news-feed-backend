package fixtures

import (
	"database/sql"
	"time"

	"github.com/nathanap/news-feed-backend/services/utctime"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestUser() db.User {
	return db.User{
		ID:          "01900000-0000-7000-8000-000000000001",
		GoogleID:    "google-test-id-123456",
		Email:       "test@example.com",
		Name:        "Test User",
		Picture:     sql.NullString{String: "https://example.com/photo.jpg", Valid: true},
		Status:      true,
		LastLoginAt: utctime.NullTime{},
		CreatedAt:   utctime.New(time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)),
		ModifiedAt:  utctime.NullTime{},
		RemovedAt:   utctime.NullTime{},
	}
}

func NewTestUserWithoutPicture() db.User {
	u := NewTestUser()
	u.Picture = sql.NullString{Valid: false}
	return u
}
