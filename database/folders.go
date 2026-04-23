package database

import "fmt"

// CountChildrenInDir returns the number of notes + task lists in a given
// parent directory. Used by buildFolderEntry to compute child_count.
func (d *Database) CountChildrenInDir(parentDir string) (int, error) {
	var count int
	err := d.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM notes WHERE parent_dir = ?) +
			(SELECT COUNT(*) FROM task_lists WHERE parent_dir = ?)`,
		parentDir, parentDir,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count children in dir: %w", err)
	}
	return count, nil
}

// DeleteByParentDir deletes all notes and task_lists rows where parent_dir
// equals dirPath or starts with dirPath + "/". Used by DeleteFolder to cascade
// database cleanup when a folder is removed from disk.
func (d *Database) DeleteByParentDir(dirPath string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	prefix := dirPath + "/"

	_, err = tx.Exec(
		`DELETE FROM notes WHERE parent_dir = ? OR parent_dir LIKE ? ESCAPE '\'`,
		dirPath, escapeLike(prefix)+"%",
	)
	if err != nil {
		return fmt.Errorf("delete notes by parent_dir: %w", err)
	}

	_, err = tx.Exec(
		`DELETE FROM task_lists WHERE parent_dir = ? OR parent_dir LIKE ? ESCAPE '\'`,
		dirPath, escapeLike(prefix)+"%",
	)
	if err != nil {
		return fmt.Errorf("delete task_lists by parent_dir: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// RenameParentDir updates parent_dir for all notes and task_lists rows where
// parent_dir equals oldPath or starts with oldPath + "/", replacing the old
// prefix with newPath. Used by UpdateFolder (rename).
func (d *Database) RenameParentDir(oldPath, newPath string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	oldPrefix := oldPath + "/"
	newPrefix := newPath + "/"
	escapedOldPrefix := escapeLike(oldPrefix)

	// Update exact matches (items directly in the renamed folder).
	_, err = tx.Exec(
		`UPDATE notes SET parent_dir = ? WHERE parent_dir = ?`,
		newPath, oldPath,
	)
	if err != nil {
		return fmt.Errorf("update notes parent_dir (exact): %w", err)
	}

	// Update prefix matches (items in subdirectories of the renamed folder).
	_, err = tx.Exec(
		`UPDATE notes SET parent_dir = ? || SUBSTR(parent_dir, ?) WHERE parent_dir LIKE ? ESCAPE '\'`,
		newPrefix, len(oldPrefix)+1, escapedOldPrefix+"%",
	)
	if err != nil {
		return fmt.Errorf("update notes parent_dir (prefix): %w", err)
	}

	// Same for task_lists.
	_, err = tx.Exec(
		`UPDATE task_lists SET parent_dir = ? WHERE parent_dir = ?`,
		newPath, oldPath,
	)
	if err != nil {
		return fmt.Errorf("update task_lists parent_dir (exact): %w", err)
	}

	_, err = tx.Exec(
		`UPDATE task_lists SET parent_dir = ? || SUBSTR(parent_dir, ?) WHERE parent_dir LIKE ? ESCAPE '\'`,
		newPrefix, len(oldPrefix)+1, escapedOldPrefix+"%",
	)
	if err != nil {
		return fmt.Errorf("update task_lists parent_dir (prefix): %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// escapeLike escapes special LIKE characters (%, _, \) in a string.
func escapeLike(s string) string {
	var result []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '%', '_', '\\':
			result = append(result, '\\')
		}
		result = append(result, s[i])
	}
	return string(result)
}
