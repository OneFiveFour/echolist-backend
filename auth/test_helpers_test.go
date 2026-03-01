package auth

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// TestMain lowers the bcrypt cost for all tests in this package so that
// property-based tests with many iterations run in seconds, not minutes.
func TestMain(m *testing.M) {
	bcryptCost = bcrypt.MinCost
	os.Exit(m.Run())
}
