package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"
)

func uuidGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`)
}

func filePathGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z0-9_]{1,30}\.md`)
}

type testRegistryEntry struct {
	FilePath string `json:"filePath"`
}

// Feature: note-registry-struct-refactor, Property 4: readRegistryReverse produces correct inverse map
// For any valid map[string]registryEntry written to disk, calling readRegistryReverse
// should produce a map where every (filePath, id) pair corresponds to an (id, entry)
// pair in the original map where entry.FilePath == filePath.
// **Validates: Requirements 4.1, 5.5**
func TestProperty_ReadRegistryReverseInverse(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dir := t.TempDir()
		regPath := filepath.Join(dir, ".registry.json")

		// Generate a registry with unique file paths to avoid collisions in the reverse map.
		m := rapid.MapOf(uuidGen(), rapid.Map(filePathGen(), func(fp string) testRegistryEntry {
			return testRegistryEntry{FilePath: fp}
		})).Draw(rt, "registry")

		data, err := json.Marshal(m)
		if err != nil {
			rt.Fatalf("json.Marshal: %v", err)
		}
		if err := os.WriteFile(regPath, data, 0644); err != nil {
			rt.Fatalf("WriteFile: %v", err)
		}

		reverse := readRegistryReverse(regPath)

		// Build the set of unique filePaths (non-empty) present in the forward map.
		uniquePaths := make(map[string]bool)
		for _, entry := range m {
			if entry.FilePath != "" {
				uniquePaths[entry.FilePath] = true
			}
		}

		if len(reverse) != len(uniquePaths) {
			rt.Fatalf("reverse map length = %d, expected %d unique filePaths", len(reverse), len(uniquePaths))
		}

		// Every entry in the reverse map must point back to a valid forward entry.
		for fp, gotID := range reverse {
			fwdEntry, ok := m[gotID]
			if !ok {
				rt.Fatalf("reverse[%q] = %q, but that id is not in the forward map", fp, gotID)
			}
			if fwdEntry.FilePath != fp {
				rt.Fatalf("reverse[%q] = %q, but forward[%q].FilePath = %q", fp, gotID, gotID, fwdEntry.FilePath)
			}
		}
	})
}
