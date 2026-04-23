package database

import (
	"database/sql"
	"fmt"
)

// migrations is an ordered list of schema migrations. Each entry is a SQL
// string that migrates from version N-1 to version N (where N is the index+1).
// New migrations are appended to the end — never modify existing entries.
var migrations = []string{
	// Version 1: initial schema
	`CREATE TABLE IF NOT EXISTS task_lists (
		id             TEXT PRIMARY KEY,
		title          TEXT NOT NULL,
		parent_dir     TEXT NOT NULL DEFAULT '',
		is_auto_delete INTEGER NOT NULL DEFAULT 0,
		created_at     INTEGER NOT NULL,
		updated_at     INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id              TEXT PRIMARY KEY,
		task_list_id    TEXT REFERENCES task_lists(id) ON DELETE CASCADE,
		parent_task_id  TEXT REFERENCES tasks(id) ON DELETE CASCADE,
		position        INTEGER NOT NULL,
		description     TEXT NOT NULL,
		is_done         INTEGER NOT NULL DEFAULT 0,
		due_date        TEXT,
		recurrence      TEXT
	);

	CREATE TABLE IF NOT EXISTS notes (
		id         TEXT PRIMARY KEY,
		title      TEXT NOT NULL,
		parent_dir TEXT NOT NULL DEFAULT '',
		preview    TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);`,
}

// createSchema reads the current schema version from the database and applies
// any pending migrations. Each migration runs in its own transaction.
func createSchema(db *sql.DB) error {
	currentVersion, err := getSchemaVersion(db)
	if err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	for i := currentVersion; i < len(migrations); i++ {
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", i+1, err)
		}

		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %d: %w", i+1, err)
		}

		// user_version cannot be set inside a transaction with a parameter,
		// so we use Sprintf here. The value is always a trusted integer.
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("set schema version %d: %w", i+1, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", i+1, err)
		}
	}

	return nil
}

// getSchemaVersion reads the current user_version from the database.
// Returns 0 for a fresh database.
func getSchemaVersion(db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}
