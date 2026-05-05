package file

import (
	"log/slog"
	"testing"

	"echolist-backend/common"
	"echolist-backend/database"
)

// NewTestDB delegates to common.NewTestDB for use in file_test package tests.
func NewTestDB(t *testing.T) *database.Database {
	return common.NewTestDB(t)
}

// NopLogger delegates to common.NopLogger for use in file_test package tests.
func NopLogger() *slog.Logger {
	return common.NopLogger()
}
