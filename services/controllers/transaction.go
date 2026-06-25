package controllers

import (
	"context"
	"database/sql"
	"fmt"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

// TransactionRunner runs fn within a database transaction. It is injected into routes,
// middleware and use cases so that production/integration/E2E receive the real
// implementation (bound to the database) while unit tests can supply a no-DB substitute.
type TransactionRunner func(ctx context.Context, fn func(q db.Querier) error) error

// NewTransactionRunner returns the real TransactionRunner bound to the database pool.
func NewTransactionRunner(database *sql.DB) TransactionRunner {
	return func(ctx context.Context, fn func(q db.Querier) error) error {
		return WithTransaction(ctx, database, fn)
	}
}

// WithTransaction opens a transaction, runs fn with a transaction-bound querier, and
// commits on success or rolls back on error. This is the single place that owns
// commit/rollback — controllers never finalize an operation on their own. They are
// intermediate steps that operate on whatever querier they receive.
func WithTransaction(ctx context.Context, database *sql.DB, fn func(q db.Querier) error) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := fn(db.New(tx)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
