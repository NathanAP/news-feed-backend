package utils

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/nathanap/news-feed-backend/schemas"
)

// ParseTestClaims decodes an access token the way the auth middleware does, so an end-to-end test can
// look inside a token it obtained through a real login — to read the user id it needs for a database
// change, or to assert on a claim the client will act upon (`admin`, for one).
func ParseTestClaims(t *testing.T, accessToken string, secret []byte) *schemas.Claims {
	t.Helper()

	claims := &schemas.Claims{}
	token, err := jwt.ParseWithClaims(accessToken, claims, func(*jwt.Token) (interface{}, error) {
		return secret, nil
	})
	require.NoError(t, err)
	require.True(t, token.Valid, "expected a valid access token")

	return claims
}
