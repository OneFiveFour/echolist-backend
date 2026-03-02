package auth

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// nopLogger returns a logger that discards all output, for use in tests.
func nopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestMain lowers the bcrypt cost for all tests in this package so that
// property-based tests with many iterations run in seconds, not minutes.
func TestMain(m *testing.M) {
	bcryptCost = bcrypt.MinCost
	os.Exit(m.Run())
}
