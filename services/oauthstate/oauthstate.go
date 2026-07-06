// Package oauthstate signs and verifies the OAuth login `state` parameter. The state carries the
// client's already-validated redirect_uri through the Google round-trip — HTTP is stateless, so the
// callback has no other way to recover where to send the user — and doubles as CSRF protection: it
// is an HMAC (HS256 JWT) signed with the server secret and short-lived, so a forged or stale
// callback fails validation. The redirect_uri is trusted on the way back only because it was
// checked against the allowlist before signing; the callback trusts the signature, not the raw URI.
package oauthstate

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Expiry bounds how long a login attempt may take between /auth/google and the callback.
const Expiry = 10 * time.Minute

// ErrInvalidState is returned when the state is missing, malformed, tampered, expired, or signed
// with a different key.
var ErrInvalidState = errors.New("invalid oauth state")

type stateClaims struct {
	RedirectURI string `json:"redirect_uri"`
	Nonce       string `json:"nonce"`
	jwt.RegisteredClaims
}

// Generate signs a state token binding the given (already allowlisted) redirect_uri, a random nonce
// for CSRF, and a short expiry.
func Generate(secret []byte, redirectURI string) (string, error) {
	nonce, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("failed to generate state nonce: %w", err)
	}

	claims := stateClaims{
		RedirectURI: redirectURI,
		Nonce:       nonce.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(Expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign oauth state: %w", err)
	}
	return signed, nil
}

// Validate verifies the signature and expiry and returns the bound redirect_uri. Any problem — bad
// signature, expired, wrong signing method, or empty redirect_uri — yields ErrInvalidState.
func Validate(secret []byte, state string) (string, error) {
	if state == "" {
		return "", ErrInvalidState
	}

	var claims stateClaims
	token, err := jwt.ParseWithClaims(state, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidState
		}
		return secret, nil
	})
	if err != nil || !token.Valid || claims.RedirectURI == "" {
		return "", ErrInvalidState
	}
	return claims.RedirectURI, nil
}
