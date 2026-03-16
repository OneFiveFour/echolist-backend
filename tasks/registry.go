package tasks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"echolist-backend/common"
)

// registryPath returns the path to the registry JSON file.
func registryPath(dataDir string) string {
	return filepath.Join(dataDir, ".tasklist_id_registry.json")
}

// registryRead reads and parses the registry from disk.
// Returns an empty map if the file is missing or empty.
func registryRead(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]string), nil
	}
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// registryWrite atomically writes the registry map to disk as JSON.
func registryWrite(path string, m map[string]string) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return common.File(path, data)
}

// registryLookup reads the registry and returns the file_path for the given id.
// Returns ("", false, nil) if not found.
func registryLookup(regPath, id string) (string, bool, error) {
	m, err := registryRead(regPath)
	if err != nil {
		return "", false, err
	}
	fp, ok := m[id]
	return fp, ok, nil
}

// registryAdd reads the registry, adds the id→filePath entry, and writes it back atomically.
func registryAdd(regPath, id, filePath string) error {
	m, err := registryRead(regPath)
	if err != nil {
		return err
	}
	m[id] = filePath
	return registryWrite(regPath, m)
}

// registryRemove reads the registry, removes the entry for id, and writes it back atomically.
func registryRemove(regPath, id string) error {
	m, err := registryRead(regPath)
	if err != nil {
		return err
	}
	delete(m, id)
	return registryWrite(regPath, m)
}
