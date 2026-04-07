package notes

import (
	"os"
	"path/filepath"
	"testing"
)

// Requirements: 2.1, 2.2, 2.3, 2.5

func TestRegistryPath(t *testing.T) {
	got := registryPath("/data")
	want := filepath.Join("/data", ".note_id_registry.json")
	if got != want {
		t.Errorf("registryPath(/data) = %q; want %q", got, want)
	}
}

func TestRegistryRead_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".note_id_registry.json")

	m, err := registryRead(path)
	if err != nil {
		t.Fatalf("registryRead on missing file: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %v", m)
	}
}

func TestRegistryRead_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".note_id_registry.json")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	m, err := registryRead(path)
	if err != nil {
		t.Fatalf("registryRead on empty file: %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %v", m)
	}
}

func TestRegistryAdd_ThenLookup(t *testing.T) {
	dir := t.TempDir()
	regPath := registryPath(dir)

	id := "550e8400-e29b-41d4-a716-446655440000"
	fp := "note_Meeting.md"

	if err := registryAdd(regPath, id, registryEntry{FilePath: fp}); err != nil {
		t.Fatalf("registryAdd: %v", err)
	}

	got, ok, err := registryLookup(regPath, id)
	if err != nil {
		t.Fatalf("registryLookup: %v", err)
	}
	if !ok {
		t.Fatal("registryLookup: expected ok=true, got false")
	}
	if got.FilePath != fp {
		t.Errorf("registryLookup FilePath = %q; want %q", got.FilePath, fp)
	}
}

func TestRegistryLookup_NotFound(t *testing.T) {
	dir := t.TempDir()
	regPath := registryPath(dir)

	_, ok, err := registryLookup(regPath, "550e8400-e29b-41d4-a716-446655440000")
	if err != nil {
		t.Fatalf("registryLookup: %v", err)
	}
	if ok {
		t.Fatal("registryLookup: expected ok=false for missing id")
	}
}

func TestRegistryRemove(t *testing.T) {
	dir := t.TempDir()
	regPath := registryPath(dir)

	id := "550e8400-e29b-41d4-a716-446655440000"
	if err := registryAdd(regPath, id, registryEntry{FilePath: "note_Test.md"}); err != nil {
		t.Fatalf("registryAdd: %v", err)
	}

	if err := registryRemove(regPath, id); err != nil {
		t.Fatalf("registryRemove: %v", err)
	}

	_, ok, err := registryLookup(regPath, id)
	if err != nil {
		t.Fatalf("registryLookup after remove: %v", err)
	}
	if ok {
		t.Fatal("expected entry to be removed, but lookup returned ok=true")
	}
}
