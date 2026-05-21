package common

import (
	"os"
	"path/filepath"
)

// atomicWrite is the shared core: write data to path via temp file + rename.
// cleanup is called on error if extra rollback is needed (e.g. removing a sentinel).
func atomicWrite(path string, data []byte, cleanup func()) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		if cleanup != nil {
			cleanup()
		}
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		if cleanup != nil {
			cleanup()
		}
		return err
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		if cleanup != nil {
			cleanup()
		}
		return err
	}

	if err := tmp.Close(); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return err
	}

	return os.Rename(tmp.Name(), path)
}

// File writes data to path atomically via temp file + rename, overwriting any
// existing file at path. Use this for updates where the file is expected to
// already exist.
func File(path string, data []byte) error {
	return atomicWrite(path, data, nil)
}

// CreateExclusive writes data to path atomically, failing with an error
// wrapping os.ErrExist if the target file already exists. The existence check
// uses O_EXCL which is atomic at the filesystem level, preventing race
// conditions where concurrent create requests could silently overwrite each
// other. Use this for initial file creation where duplicates must be rejected.
func CreateExclusive(path string, data []byte) error {
	sentinel, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	sentinel.Close()

	return atomicWrite(path, data, func() { os.Remove(path) })
}
