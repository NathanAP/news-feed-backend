package oauthstate

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSecret = []byte("test-secret-key")

func TestGenerateValidateRoundtrip(t *testing.T) {
	redirectURI := "http://localhost:5173/auth/callback"

	state, err := Generate(testSecret, redirectURI)
	require.NoError(t, err)
	require.NotEmpty(t, state)

	got, err := Validate(testSecret, state)
	require.NoError(t, err)
	assert.Equal(t, redirectURI, got)
}

func TestValidate_WrongSecret(t *testing.T) {
	state, err := Generate(testSecret, "http://localhost:5173/auth/callback")
	require.NoError(t, err)

	_, err = Validate([]byte("another-secret"), state)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestValidate_Tampered(t *testing.T) {
	state, err := Generate(testSecret, "http://localhost:5173/auth/callback")
	require.NoError(t, err)

	_, err = Validate(testSecret, state+"x")
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestValidate_Empty(t *testing.T) {
	_, err := Validate(testSecret, "")
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestValidate_Expired(t *testing.T) {
	// Hand-craft an expired token signed with the right key.
	claims := stateClaims{
		RedirectURI: "http://localhost:5173/auth/callback",
		Nonce:       "n",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(testSecret)
	require.NoError(t, err)

	_, err = Validate(testSecret, signed)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestValidate_EmptyRedirectURI(t *testing.T) {
	state, err := Generate(testSecret, "")
	require.NoError(t, err)

	_, err = Validate(testSecret, state)
	assert.ErrorIs(t, err, ErrInvalidState)
}
