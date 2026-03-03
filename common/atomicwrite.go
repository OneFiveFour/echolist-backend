package common

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
func CreateExclusive(path string, data []byte) error {
	dir := filepath.Dir(path)

	sentinel, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	sentinel.Close()

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		os.Remove(path)
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
