package notes

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"echolist-backend/common"
)

// registryEntry holds the metadata for a single note in the registry.
type registryEntry struct {
	FilePath string `json:"filePath"`
}

// registryPath returns the path to the registry JSON file.
func registryPath(dataDir string) string {
	return filepath.Join(dataDir, ".note_id_registry.json")
}

// registryRead reads and parses the registry from disk.
// Returns an empty map if the file is missing or empty.
func registryRead(path string) (map[string]registryEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return make(map[string]registryEntry), nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return make(map[string]registryEntry), nil
	}
	m := make(map[string]registryEntry)
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// registryWrite atomically writes the registry map to disk as JSON.
func registryWrite(path string, m map[string]registryEntry) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return common.File(path, data)
}

// registryLookup reads the registry and returns the entry for the given id.
// Returns (registryEntry{}, false, nil) if not found.
func registryLookup(regPath, id string) (registryEntry, bool, error) {
	m, err := registryRead(regPath)
	if err != nil {
		return registryEntry{}, false, err
	}
	entry, ok := m[id]
	return entry, ok, nil
}

// registryAdd reads the registry, adds the id→entry mapping, and writes it back atomically.
func registryAdd(regPath, id string, entry registryEntry) error {
	m, err := registryRead(regPath)
	if err != nil {
		return err
	}
	m[id] = entry
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
