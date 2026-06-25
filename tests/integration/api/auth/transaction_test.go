package auth_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
	_ "modernc.org/sqlite"
)

// TestWithTransaction_Integration_RollsBackOnError proves the orphan-prevention guarantee:
// if a step fails after an earlier write inside the same transaction, the earlier write is
// rolled back and nothing is persisted.
func TestWithTransaction_Integration_RollsBackOnError(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	runTx := controllers.NewTransactionRunner(database)
	userCtrl := controllers.NewUserController()
	queries := db.New(database)

	forcedErr := errors.New("forced failure after the user was written")

	err := runTx(context.Background(), func(q db.Querier) error {
		if _, err := userCtrl.CreateUser(context.Background(), q, "google-rollback", "rollback@example.com", "Rollback User", ""); err != nil {
			return err
		}
		// Simulate a later step in the flow failing.
		return forcedErr
	})
	require.ErrorIs(t, err, forcedErr)

	// Neither the user nor its preferences should exist — the whole transaction rolled back.
	_, err = queries.FindUserByGoogleID(context.Background(), "google-rollback")
	assert.ErrorIs(t, err, sql.ErrNoRows, "user must not persist after a rolled-back transaction")
}

// TestWithTransaction_Integration_CommitsOnSuccess proves that a multi-write flow persists
// every write when the transaction completes successfully.
func TestWithTransaction_Integration_CommitsOnSuccess(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	runTx := controllers.NewTransactionRunner(database)
	userCtrl := controllers.NewUserController()
	queries := db.New(database)

	var created db.User
	err := runTx(context.Background(), func(q db.Querier) error {
		var err error
		created, err = userCtrl.CreateUser(context.Background(), q, "google-commit", "commit@example.com", "Commit User", "")
		return err
	})
	require.NoError(t, err)

	got, err := queries.FindUserByID(context.Background(), created.ID)
	require.NoError(t, err)
	assert.Equal(t, "commit@example.com", got.Email)

	prefs, err := queries.FindUserPreferencesByUserID(context.Background(), created.ID)
	require.NoError(t, err, "preferences must persist alongside the user")
	assert.Equal(t, "dark", prefs.Theme)
}

// TestCreateUser_Integration_DuplicateReturnsError covers the write-failure path where the
// user insert itself fails on a UNIQUE constraint.
func TestCreateUser_Integration_DuplicateReturnsError(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	runTx := controllers.NewTransactionRunner(database)
	userCtrl := controllers.NewUserController()
	queries := db.New(database)

	err := runTx(context.Background(), func(q db.Querier) error {
		_, err := userCtrl.CreateUser(context.Background(), q, "google-dup", "dup@example.com", "Dup User", "")
		return err
	})
	require.NoError(t, err)

	// Same google_id again → UNIQUE violation surfaced as a typed error, nothing new persisted.
	err = runTx(context.Background(), func(q db.Querier) error {
		_, err := userCtrl.CreateUser(context.Background(), q, "google-dup", "dup2@example.com", "Dup User 2", "")
		return err
	})
	require.ErrorIs(t, err, controllers.ErrUserAlreadyExists)

	// The original user is still intact.
	original, err := queries.FindUserByGoogleID(context.Background(), "google-dup")
	require.NoError(t, err)
	assert.Equal(t, "dup@example.com", original.Email)
}
