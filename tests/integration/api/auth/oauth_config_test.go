package auth_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuthConfig_Integration_RequiredEnvVarsPresent(t *testing.T) {
	requireNotProduction(t)

	assert.NotEmpty(t, os.Getenv("GOOGLE_CLIENT_ID"),
		"GOOGLE_CLIENT_ID must be set for OAuth2 to work")
	assert.NotEmpty(t, os.Getenv("GOOGLE_CLIENT_SECRET"),
		"GOOGLE_CLIENT_SECRET must be set for OAuth2 to work")
	assert.NotEmpty(t, os.Getenv("GOOGLE_REDIRECT_URL"),
		"GOOGLE_REDIRECT_URL must be set for OAuth2 to work")
}
