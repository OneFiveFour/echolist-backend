package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Requirements: 2.1, 2.2, 2.3, 2.4, 2.5

func TestRegistryPath(t *testing.T) {
	got := registryPath("/data")
	want := filepath.Join("/data", ".tasklist_id_registry.json")
	if got != want {
		t.Errorf("registryPath(/data) = %q; want %q", got, want)
	}
}

func TestRegistryRead_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".tasklist_id_registry.json")

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
	path := filepath.Join(dir, ".tasklist_id_registry.json")
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

func TestRegistryRead_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".tasklist_id_registry.json")
	data := `{"550e8400-e29b-41d4-a716-446655440000":{"filePath":"tasks_Groceries.md","isAutoDelete":false}}`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := registryRead(path)
	if err != nil {
		t.Fatalf("registryRead on valid JSON: %v", err)
	}
	if len(m) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(m))
	}
	entry := m["550e8400-e29b-41d4-a716-446655440000"]
	if entry.FilePath != "tasks_Groceries.md" {
		t.Errorf("unexpected FilePath: %v", entry.FilePath)
	}
	if entry.IsAutoDelete {
		t.Errorf("expected IsAutoDelete=false, got true")
	}
}

func TestRegistryRead_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".tasklist_id_registry.json")
	if err := os.WriteFile(path, []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := registryRead(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestRegistryWrite_CreatesValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".tasklist_id_registry.json")

	m := map[string]registryEntry{
		"550e8400-e29b-41d4-a716-446655440000": {FilePath: "tasks_Meeting.md", IsAutoDelete: true},
	}
	if err := registryWrite(path, m); err != nil {
		t.Fatalf("registryWrite: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written file: %v", err)
	}

	var got map[string]registryEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("written file is not valid JSON: %v", err)
	}
	entry := got["550e8400-e29b-41d4-a716-446655440000"]
	if entry.FilePath != "tasks_Meeting.md" {
		t.Errorf("unexpected FilePath: %v", entry.FilePath)
	}
	if !entry.IsAutoDelete {
		t.Errorf("expected IsAutoDelete=true, got false")
	}
}

func TestRegistryAdd_ThenLookup(t *testing.T) {
	dir := t.TempDir()
	regPath := registryPath(dir)

	id := "550e8400-e29b-41d4-a716-446655440000"
	entry := registryEntry{FilePath: "tasks_Meeting.md"}

	if err := registryAdd(regPath, id, entry); err != nil {
		t.Fatalf("registryAdd: %v", err)
	}

	got, ok, err := registryLookup(regPath, id)
	if err != nil {
		t.Fatalf("registryLookup: %v", err)
	}
	if !ok {
		t.Fatal("registryLookup: expected ok=true, got false")
	}
	if got.FilePath != entry.FilePath {
		t.Errorf("registryLookup FilePath = %q; want %q", got.FilePath, entry.FilePath)
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
	if err := registryAdd(regPath, id, registryEntry{FilePath: "tasks_Test.md"}); err != nil {
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
