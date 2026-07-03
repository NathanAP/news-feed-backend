package main

import (
	"errors"
	"fmt"

	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// requireDevUser fetches the development user (by its fixed google_id) or returns a clear error
// telling the operator to seed it first. Shared by the commands that need the dev user.
func requireDevUser(sc *seedCtx, q db.Querier) (db.User, error) {
	user, err := sc.userCtrl.FindUserByGoogleID(sc.ctx, q, schemas.DevUserGoogleID)
	if err != nil {
		if errors.Is(err, controllers.ErrUserNotFound) {
			return db.User{}, fmt.Errorf("dev user not found — run `task sud` (seed-user-dev) first")
		}
		return db.User{}, err
	}
	return user, nil
}

// runDevLogin mints a fresh refresh_token + access_token for the dev user and prints them, so the
// operator can paste them into Bruno / the client without going through Google OAuth. Mirrors the
// dev-login endpoint. The dev user must already exist.
func runDevLogin(sc *seedCtx) (*report, error) {
	rep := &report{}
	var accessToken, refreshTokenID string

	err := sc.runTx(sc.ctx, func(q db.Querier) error {
		user, err := requireDevUser(sc, q)
		if err != nil {
			return err
		}

		prefs, err := sc.prefCtrl.FindByUserID(sc.ctx, q, user.ID)
		if err != nil {
			return err
		}

		rt, err := sc.refreshCtrl.Create(sc.ctx, q, user.ID)
		if err != nil {
			return err
		}

		token, err := sc.authCtrl.GenerateAccessToken(user, rt.ID, prefs)
		if err != nil {
			return err
		}

		accessToken = token
		refreshTokenID = rt.ID
		return nil
	})
	if err != nil {
		return rep, err
	}

	// Tokens are dev-only credentials for local use; printing them to the terminal is the point.
	fmt.Println("\n=== dev login ===")
	fmt.Printf("access_token:  %s\n", accessToken)
	fmt.Printf("refresh_token: %s\n", refreshTokenID)

	rep.add("issued access_token + refresh_token for the dev user")
	return rep, nil
}
