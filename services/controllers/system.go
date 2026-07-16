package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nathanap/news-feed-backend/services/utctime"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

var ErrSystemNotFound = errors.New("system record not found")

// SystemController exposes the singleton control-panel row. The table is never inserted into
// or deleted from here: it is seeded by the migration and only ever read or updated.
type SystemController struct{}

func NewSystemController() *SystemController {
	return &SystemController{}
}

// Get returns the singleton system row.
func (c *SystemController) Get(ctx context.Context, q db.Querier) (db.System, error) {
	system, err := q.GetSystem(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.System{}, ErrSystemNotFound
		}
		return db.System{}, fmt.Errorf("failed to get system: %w", err)
	}
	return system, nil
}

// UpdateAppStatus flips the global maintenance switch on the singleton row.
func (c *SystemController) UpdateAppStatus(ctx context.Context, q db.Querier, active bool) (db.System, error) {
	system, err := q.UpdateSystemAppStatus(ctx, active)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.System{}, ErrSystemNotFound
		}
		return db.System{}, fmt.Errorf("failed to update app status: %w", err)
	}
	return system, nil
}

// UpdateLastArticleDiscovery records the moment the discovery CRON finished a run. The value is
// the lower bound used to decide what counts as "new" on the next run.
func (c *SystemController) UpdateLastArticleDiscovery(ctx context.Context, q db.Querier, at time.Time) (db.System, error) {
	system, err := q.UpdateSystemLastArticleDiscovery(ctx, utctime.NewNull(at))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.System{}, ErrSystemNotFound
		}
		return db.System{}, fmt.Errorf("failed to update last article discovery: %w", err)
	}
	return system, nil
}
