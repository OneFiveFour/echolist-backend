package tasks

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"echolist-backend/database"
)

// NewTestDB creates an in-memory SQLite database with full schema for use in tests.
// It registers a cleanup function to close the database when the test completes.
func NewTestDB(t *testing.T) *database.Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// NopLogger returns a logger that discards all output, for use in tests.
func NopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
