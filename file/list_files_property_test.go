package file

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	filev1 "echolist-backend/proto/gen/file/v1"
	"pgregory.net/rapid"
)

// Feature: list-files-enrichment, Property: ListFiles filter correctness
// **Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5**
func TestProperty_ListFilesFilterCorrectness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, nopLogger())

		// Track what we create so we can compute expected results
		type entry struct {
			name  string
			isDir bool
		}
		var created []entry
		usedNames := make(map[string]bool)

		// Generate a random mix of note_ files, tasks_ files, other-prefixed files, and subdirectories
		numEntries := rapid.IntRange(0, 15).Draw(rt, "numEntries")

		prefixGen := rapid.SampledFrom([]string{"note_", "tasks_", "other_", "readme_", "config_", ""})
		suffixGen := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`)

		for i := 0; i < numEntries; i++ {
			kind := rapid.SampledFrom([]string{"file", "dir"}).Draw(rt, "kind")
			prefix := prefixGen.Draw(rt, "prefix")
			suffix := suffixGen.Draw(rt, "suffix")

			var name string
			if kind == "dir" {
				name = suffix
			} else {
				name = prefix + suffix + ".md"
			}

			lower := strings.ToLower(name)
			if usedNames[lower] || name == "" {
				continue
			}
			usedNames[lower] = true

			fullPath := filepath.Join(dataDir, name)
			if kind == "dir" {
				if err := os.Mkdir(fullPath, 0755); err != nil {
					continue
				}
				created = append(created, entry{name: name, isDir: true})
			} else {
				if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
					continue
				}
				created = append(created, entry{name: name, isDir: false})
			}
		}

		// Call ListFiles
		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Compute expected entries: note_ files, tasks_ files, and all directories
		var expected []string
		for _, e := range created {
			if e.isDir {
				expected = append(expected, e.name)
			} else if strings.HasPrefix(e.name, "note_") || strings.HasPrefix(e.name, "tasks_") {
				expected = append(expected, e.name)
			}
			// other-prefixed files should be excluded
		}

		sort.Strings(expected)
		gotPaths := make([]string, len(resp.Entries))
		for i, entry := range resp.Entries {
			gotPaths[i] = entry.Path
		}
		sort.Strings(gotPaths)

		// Verify exact match
		if len(gotPaths) != len(expected) {
			rt.Fatalf("expected %d entries, got %d\nexpected: %v\ngot: %v", len(expected), len(gotPaths), expected, gotPaths)
		}
		for i := range expected {
			if gotPaths[i] != expected[i] {
				rt.Fatalf("entry mismatch at %d: expected %q, got %q\nexpected: %v\ngot: %v", i, expected[i], gotPaths[i], expected, gotPaths)
			}
		}

		// Verify no other-prefixed files leaked through and types are correct
		for _, e := range resp.Entries {
			if e.ItemType == filev1.ItemType_ITEM_TYPE_FOLDER {
				// Folders are always included
				continue
			}
			if e.ItemType == filev1.ItemType_ITEM_TYPE_NOTE && !strings.HasPrefix(e.Path, "note_") {
				rt.Fatalf("NOTE entry %q should have note_ prefix", e.Path)
			}
			if e.ItemType == filev1.ItemType_ITEM_TYPE_TASK_LIST && !strings.HasPrefix(e.Path, "tasks_") {
				rt.Fatalf("TASK_LIST entry %q should have tasks_ prefix", e.Path)
			}
			if e.ItemType != filev1.ItemType_ITEM_TYPE_NOTE && e.ItemType != filev1.ItemType_ITEM_TYPE_TASK_LIST && e.ItemType != filev1.ItemType_ITEM_TYPE_FOLDER {
				rt.Fatalf("unexpected item type %v for entry %q", e.ItemType, e.Path)
			}
		}
	})
}
