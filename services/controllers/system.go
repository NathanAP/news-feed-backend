package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	status := int64(0)
	if active {
		status = 1
	}

	system, err := q.UpdateSystemAppStatus(ctx, status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.System{}, ErrSystemNotFound
		}
		return db.System{}, fmt.Errorf("failed to update app status: %w", err)
	}
	return system, nil
}
