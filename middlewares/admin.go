package middlewares

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/nathanap/news-feed-backend/services/controllers"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

// AdminResolver answers one question — "is the caller an administrator?" — always against the
// database, never against the token.
//
// The access token does carry an `admin` claim, but only so the client can decide what to render.
// Trusting it here would mean a revoked administrator keeps administrator powers until their token
// expires (up to JWT_ACCESS_TOKEN_EXPIRY_MINUTES), and revoking someone's access is exactly the
// moment when waiting an hour is unacceptable. The cost of reading the database instead is one query,
// paid only on the admin-only routes (low traffic by nature) and while the app is under maintenance.
type AdminResolver struct {
	jwtSecret        []byte
	refreshTokenCtrl controllers.RefreshTokenControllerInterface
	userCtrl         controllers.UserControllerInterface
	runTx            controllers.TransactionRunner
}

func NewAdminResolver(
	jwtSecret []byte,
	refreshTokenCtrl controllers.RefreshTokenControllerInterface,
	userCtrl controllers.UserControllerInterface,
	runTx controllers.TransactionRunner,
) *AdminResolver {
	return &AdminResolver{
		jwtSecret:        jwtSecret,
		refreshTokenCtrl: refreshTokenCtrl,
		userCtrl:         userCtrl,
		runTx:            runTx,
	}
}

// IsAdmin reports whether the user behind an already-authenticated request is an administrator. The
// returned error is reserved for infrastructure failures: a user who simply is not an administrator
// (or whose row is gone) comes back as false with a nil error, so the caller can tell "denied" apart
// from "could not find out".
func (r *AdminResolver) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var user db.User
	err := r.runTx(ctx, func(q db.Querier) error {
		var findErr error
		user, findErr = r.userCtrl.FindUserByID(ctx, q, userID)
		return findErr
	})
	if err != nil {
		// A token whose user no longer exists (soft-removed) is not an infrastructure problem, it is
		// simply not an administrator. In practice this is unreachable — soft-removing a user revokes
		// their refresh tokens, so the session check rejects them first — but resolving it to false
		// keeps the failure closed rather than depending on that ordering holding forever.
		if errors.Is(err, controllers.ErrUserNotFound) {
			return false, nil
		}
		return false, err
	}

	return user.Admin, nil
}

// IsRequestFromAdmin identifies an administrator on a request that has *not* passed through the auth
// middleware yet. Only the maintenance guard needs this: it is mounted globally, ahead of every
// per-route authentication, so by the time it decides between 503 and letting the request through
// there are no claims in the context to read.
//
// It is deliberately best-effort and fail-closed: a missing header, a bad token, a dead session or a
// database error all resolve to "not an administrator". The whole application is already unavailable
// when this runs, so refusing to guess is the only safe answer.
func (r *AdminResolver) IsRequestFromAdmin(c fiber.Ctx) bool {
	claims, err := parseAccessToken(c.Get("Authorization"), r.jwtSecret)
	if err != nil {
		return false
	}

	// The session check matters as much as the signature: without it, an administrator who logged out
	// (or had their session invalidated) would still walk past a maintenance window with a token that
	// merely has not expired yet.
	if err := checkSession(c.Context(), r.refreshTokenCtrl, r.runTx, claims.RefreshTokenID); err != nil {
		return false
	}

	admin, err := r.IsAdmin(c.Context(), claims.UserID)
	if err != nil {
		return false
	}

	return admin
}

// NewRequireAdminMiddleware guards the routes PROJECT.md reserves for administrators. It must be
// mounted *after* the auth middleware, which is what puts the claims in the context: this middleware
// answers "may this authenticated user do it?", not "who is this?".
func NewRequireAdminMiddleware(resolver *AdminResolver) fiber.Handler {
	return func(c fiber.Ctx) error {
		claims := GetClaims(c)

		admin, err := resolver.IsAdmin(c.Context(), claims.UserID)
		if err != nil {
			// A database failure is never a permission answer. Returning 403 here would tell a
			// legitimate administrator they lost their access because the database blinked — the same
			// trap the session check avoids by not turning infrastructure errors into 401.
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "internal server error",
			})
		}

		if !admin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "forbidden: administrator access required",
			})
		}

		return c.Next()
	}
}
