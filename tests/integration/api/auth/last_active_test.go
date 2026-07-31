package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
	jwtmock "github.com/nathanap/news-feed-backend/tests/mocks/services"
	testutils "github.com/nathanap/news-feed-backend/tests/utils"
)

// TestRefresh_Integration_UpdatesLastActive proves the heartbeat that the whole inactivity filter
// depends on: a token refresh must stamp last_active_at to "now". Without this write, every user
// would drift into "inactive" and stop receiving news even while actively using the app.
func TestRefresh_Integration_UpdatesLastActive(t *testing.T) {
	requireNotProduction(t)

	database := testutils.SetupTestDB(t)
	runTx := controllers.NewTransactionRunner(database)
	queries := db.New(database)
	userCtrl := controllers.NewUserController()
	prefCtrl := controllers.NewUserPreferencesController()
	refreshTokenCtrl := controllers.NewRefreshTokenController(30 * 24 * time.Hour)
	authCtrl := controllers.NewAuthController(
		&oauth2.Config{}, userCtrl, refreshTokenCtrl, prefCtrl, runTx, []byte(jwtmock.TestJWTSecret), time.Hour,
	)

	var user db.User
	var rt db.RefreshToken
	require.NoError(t, runTx(context.Background(), func(q db.Querier) error {
		var e error
		user, e = userCtrl.CreateUser(context.Background(), q, "google-active", "active@example.com", "Active User", "")
		if e != nil {
			return e
		}
		rt, e = refreshTokenCtrl.Create(context.Background(), q, user.ID)
		return e
	}))

	// Push the activity stamp far into the past — as if the user had been away.
	_, err := database.ExecContext(context.Background(),
		"UPDATE users SET last_active_at = $1 WHERE id = $2",
		time.Now().UTC().AddDate(0, 0, -40), user.ID)
	require.NoError(t, err)

	// The refresh is the heartbeat: it must bring last_active_at back to ~now.
	_, err = authCtrl.RefreshAccessToken(context.Background(), rt.ID)
	require.NoError(t, err)

	got, err := queries.FindUserByID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().UTC(), got.LastActiveAt.Time, time.Minute,
		"refresh must stamp last_active_at to now")
}
