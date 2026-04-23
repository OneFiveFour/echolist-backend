package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Database wraps a SQLite database connection and provides typed query methods
// for task lists, tasks, and note metadata.
type Database struct {
	db *sql.DB
}

// New opens (or creates) the SQLite database at dbPath, enables WAL mode and
// foreign keys, runs idempotent schema creation, and returns a Database.
func New(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for concurrent readers with a single writer.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign key enforcement (off by default in SQLite).
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := createSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	return &Database{db: db}, nil
}

// Close closes the underlying database connection.
func (d *Database) Close() error {
	return d.db.Close()
}

// HealthCheck runs "SELECT 1" to verify the database is accessible.
func (d *Database) HealthCheck() error {
	_, err := d.db.Exec("SELECT 1")
	return err
}
