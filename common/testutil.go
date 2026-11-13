package common

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"echolist-backend/database"
)

// NewTestDB creates a SQLite database with full schema for use in tests.
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

// ResolvedTempDir returns a fresh temp directory for use as a server data root,
// with symlinks resolved. This mirrors the boot-time resolution main.go
// performs via filepath.EvalSymlinks, which path validation relies on
// (validatePath assumes dataDir is already resolved). Tests must use this
// instead of t.TempDir() directly: on macOS t.TempDir() returns a path under
// /var/folders/... that symlinks to /private/var/folders/..., so an unresolved
// data dir makes every parent_dir check fail with "path escapes data directory".
//
// It is not named with a Test prefix so the go test runner does not mistake it
// for a test function.
func ResolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("failed to resolve test data dir: %v", err)
	}
	return dir
}
