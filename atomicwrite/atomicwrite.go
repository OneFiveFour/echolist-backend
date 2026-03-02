package atomicwrite

import (
	"os"
	"path/filepath"
)

// File writes data to path atomically via temp file + rename.
func File(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), path)
}

// CreateExclusive writes data to path atomically, failing with an error
// wrapping os.ErrExist if the target file already exists.
// It uses O_CREATE|O_EXCL on the final path to avoid TOCTOU races,
// then writes via a temp file and renames into place.
func CreateExclusive(path string, data []byte) error {
	dir := filepath.Dir(path)

	// Acquire exclusive right to this path by creating a zero-byte
	// sentinel with O_EXCL. This is atomic at the filesystem level.
	sentinel, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err // wraps os.ErrExist when file already exists
	}
	sentinel.Close()

	// Write the real content via temp-file + rename.
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		os.Remove(path) // clean up sentinel
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(path)
		return err
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(path)
		return err
	}

	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return err
	}

	return os.Rename(tmp.Name(), path)
}

