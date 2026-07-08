package fixtures

import (
	"database/sql"
	"time"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

func NewTestSource() db.Source {
	return db.Source{
		ID:         "01900000-0000-7000-8000-000000000010",
		Status:     1,
		Name:       "Test Source",
		Url:        "https://example.com",
		UrlRss:     "https://example.com/rss.xml",
		CreatedAt:  time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
		ModifiedAt: sql.NullTime{Valid: false},
		RemovedAt:  sql.NullTime{Valid: false},
	}
}

func NewTestSourceAlt() db.Source {
	return db.Source{
		ID:         "01900000-0000-7000-8000-000000000011",
		Status:     1,
		Name:       "Other Source",
		Url:        "https://other-source.com",
		UrlRss:     "https://other-source.com/feed.xml",
		CreatedAt:  time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC),
		ModifiedAt: sql.NullTime{Valid: false},
		RemovedAt:  sql.NullTime{Valid: false},
	}
}
