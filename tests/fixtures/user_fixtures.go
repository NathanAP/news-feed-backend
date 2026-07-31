package fixtures

import (
	"context"
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
		// Active "now" by default so a fixture user is never treated as inactive by the discovery
		// filter. Tests that need an inactive user set LastActiveAt to a past date explicitly.
		LastActiveAt: utctime.New(time.Now().UTC()),
	}
}

func NewTestUserWithoutPicture() db.User {
	u := NewTestUser()
	u.Picture = sql.NullString{Valid: false}
	return u
}

// NewTestAdminUser is a second, distinct user carrying the admin flag. It has its own id, google_id
// and email so it can coexist with NewTestUser in the same database — the admin-only tests almost
// always need both a regular user and an administrator at once, to check the two answers of the same
// route.
func NewTestAdminUser() db.User {
	u := NewTestUser()
	u.ID = "01900000-0000-7000-8000-000000000002"
	u.GoogleID = "google-test-admin-id-123456"
	u.Email = "admin@example.com"
	u.Name = "Test Admin"
	u.Admin = true
	return u
}

// CreateUser persists a user fixture, including the admin flag. The flag needs a second statement
// because CreateUser (the query) deliberately cannot write it — keeping the login path incapable of
// producing an administrator — so tests would otherwise silently seed a regular user while believing
// they had seeded an administrator.
func CreateUser(ctx context.Context, q db.Querier, user db.User) (db.User, error) {
	created, err := q.CreateUser(ctx, db.CreateUserParams{
		ID:       user.ID,
		GoogleID: user.GoogleID,
		Email:    user.Email,
		Name:     user.Name,
		Picture:  user.Picture,
	})
	if err != nil {
		return db.User{}, err
	}

	if !user.Admin {
		return created, nil
	}

	return q.SetUserAdmin(ctx, db.SetUserAdminParams{ID: created.ID, Admin: true})
}
