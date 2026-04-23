package database

import (
	"database/sql"
	"fmt"
)

// InsertNote inserts a note metadata row. Called after the file is created on disk.
func (d *Database) InsertNote(params InsertNoteParams) error {
	_, err := d.db.Exec(
		`INSERT INTO notes (id, title, parent_dir, preview, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		params.Id, params.Title, params.ParentDir,
		params.Preview, params.CreatedAt, params.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert note: %w", err)
	}
	return nil
}

// GetNote retrieves note metadata by ID. Returns ErrNotFound if the ID does
// not exist.
func (d *Database) GetNote(id string) (NoteRow, error) {
	var n NoteRow
	err := d.db.QueryRow(
		`SELECT id, title, parent_dir, preview, created_at, updated_at
		 FROM notes WHERE id = ?`, id,
	).Scan(&n.Id, &n.Title, &n.ParentDir, &n.Preview, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return NoteRow{}, ErrNotFound
		}
		return NoteRow{}, fmt.Errorf("query note: %w", err)
	}
	return n, nil
}

// UpdateNote updates note metadata (title, preview, updated_at). Called after
// the file is renamed/written on disk.
func (d *Database) UpdateNote(id string, title string, preview string, updatedAt int64) error {
	result, err := d.db.Exec(
		`UPDATE notes SET title = ?, preview = ?, updated_at = ? WHERE id = ?`,
		title, preview, updatedAt, id,
	)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteNote deletes a note metadata row by ID. Returns false if not found.
func (d *Database) DeleteNote(id string) (bool, error) {
	result, err := d.db.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete note: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n > 0, nil
}

// ListNotes returns all note metadata rows for a given parent directory.
func (d *Database) ListNotes(parentDir string) ([]NoteRow, error) {
	sqlRows, err := d.db.Query(
		`SELECT id, title, parent_dir, preview, created_at, updated_at
		 FROM notes WHERE parent_dir = ?`, parentDir,
	)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer sqlRows.Close()

	var notes []NoteRow
	for sqlRows.Next() {
		var n NoteRow
		if err := sqlRows.Scan(&n.Id, &n.Title, &n.ParentDir, &n.Preview, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		notes = append(notes, n)
	}
	if err := sqlRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notes: %w", err)
	}

	return notes, nil
}
