package auth_test

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	// Load .env so integration tests can verify environment config variables.
	// Ignore error — in CI, env vars are set directly without a .env file.
	godotenv.Load("../../../../.env")

	os.Exit(m.Run())
}
