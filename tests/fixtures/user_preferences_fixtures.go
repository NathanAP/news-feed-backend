package fixtures

import (
	"database/sql"
	"time"

	"github.com/nathanap/news-feed-backend/services/utctime"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestUserPreferences(userID string) db.UserPreference {
	return db.UserPreference{
		ID:                  "01900000-0000-7000-8000-000000000003",
		UserID:              userID,
		Status:              true,
		LanguageToTranslate: sql.NullString{String: "pt", Valid: true},
		AiPersonality:       "mixed",
		CreatedAt:           utctime.New(time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)),
		ModifiedAt:          utctime.NullTime{},
		RemovedAt:           utctime.NullTime{},
	}
}
