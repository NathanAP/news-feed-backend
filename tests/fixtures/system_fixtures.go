package fixtures

import (
	"context"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

// SetAppStatus flips the global app_status flag on the singleton system row. The row already
// exists (seeded by the migration), so this only ever updates it. Tests use it to simulate the
// application going under maintenance (false) or being brought back online (true).
func SetAppStatus(ctx context.Context, q db.Querier, active bool) error {
	status := int64(0)
	if active {
		status = 1
	}
	_, err := q.UpdateSystemAppStatus(ctx, status)
	return err
}
