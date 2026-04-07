package notes

import (
	"path/filepath"
	"testing"

	"pgregory.net/rapid"
)

// Generators for registry property tests.

func uuidGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`)
}

func filePathGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z0-9_]{1,30}\.md`)
}

func entryGen() *rapid.Generator[registryEntry] {
	return rapid.Map(filePathGen(), func(fp string) registryEntry {
		return registryEntry{FilePath: fp}
	})
}

// Feature: note-registry-struct-refactor, Property 1: Registry write-then-read round trip
// For any valid map[string]registryEntry, writing it with registryWrite and reading
// it back with registryRead should produce an identical map.
// **Validates: Requirements 2.1, 2.2, 5.4**
func TestProperty_RegistryWriteReadRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dir := t.TempDir()
		regPath := filepath.Join(dir, ".note_id_registry.json")

		m := rapid.MapOf(uuidGen(), entryGen()).Draw(rt, "registry")

		if err := registryWrite(regPath, m); err != nil {
			rt.Fatalf("registryWrite: %v", err)
		}

		got, err := registryRead(regPath)
		if err != nil {
			rt.Fatalf("registryRead: %v", err)
		}

		if len(got) != len(m) {
			rt.Fatalf("length mismatch: wrote %d entries, read %d", len(m), len(got))
		}
		for id, want := range m {
			actual, ok := got[id]
			if !ok {
				rt.Fatalf("missing key %q after round trip", id)
			}
			if actual.FilePath != want.FilePath {
				rt.Fatalf("key %q: FilePath = %q, want %q", id, actual.FilePath, want.FilePath)
			}
		}
	})
}

// Feature: note-registry-struct-refactor, Property 2: Add-then-lookup returns the entry
// For any registry file and any new (id, registryEntry) pair, calling registryAdd
// followed by registryLookup with the same id should return the added entry with found=true.
// **Validates: Requirements 2.3, 2.4**
func TestProperty_RegistryAddThenLookup(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dir := t.TempDir()
		regPath := registryPath(dir)

		// Optionally seed with pre-existing entries.
		seed := rapid.MapOf(uuidGen(), entryGen()).Draw(rt, "seed")
		if len(seed) > 0 {
			if err := registryWrite(regPath, seed); err != nil {
				rt.Fatalf("seed registryWrite: %v", err)
			}
		}

		id := uuidGen().Draw(rt, "id")
		entry := entryGen().Draw(rt, "entry")

		if err := registryAdd(regPath, id, entry); err != nil {
			rt.Fatalf("registryAdd: %v", err)
		}

		got, found, err := registryLookup(regPath, id)
		if err != nil {
			rt.Fatalf("registryLookup: %v", err)
		}
		if !found {
			rt.Fatal("registryLookup: expected found=true after add")
		}
		if got.FilePath != entry.FilePath {
			rt.Fatalf("FilePath = %q, want %q", got.FilePath, entry.FilePath)
		}
	})
}

// Feature: note-registry-struct-refactor, Property 3: Remove-then-lookup returns not found
// For any registry containing at least one entry, calling registryRemove for a known id
// and then registryLookup for that same id should return found=false.
// **Validates: Requirements 2.5**
func TestProperty_RegistryRemoveThenLookup(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dir := t.TempDir()
		regPath := registryPath(dir)

		// Seed with at least one entry.
		id := uuidGen().Draw(rt, "id")
		entry := entryGen().Draw(rt, "entry")
		if err := registryAdd(regPath, id, entry); err != nil {
			rt.Fatalf("registryAdd: %v", err)
		}

		// Optionally add more entries.
		extras := rapid.MapOf(uuidGen(), entryGen()).Draw(rt, "extras")
		for eid, eentry := range extras {
			if err := registryAdd(regPath, eid, eentry); err != nil {
				rt.Fatalf("registryAdd extra: %v", err)
			}
		}

		if err := registryRemove(regPath, id); err != nil {
			rt.Fatalf("registryRemove: %v", err)
		}

		_, found, err := registryLookup(regPath, id)
		if err != nil {
			rt.Fatalf("registryLookup: %v", err)
		}
		if found {
			rt.Fatal("registryLookup: expected found=false after remove")
		}
	})
}
