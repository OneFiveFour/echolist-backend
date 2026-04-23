package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"connectrpc.com/connect"

	filev1 "echolist-backend/proto/gen/file/v1"
	"pgregory.net/rapid"
)

// Feature: list-files-enrichment, Property 1: ListFiles returns immediate children with correct entry format
// Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5, 9.1, 9.2
func TestProperty1_ListFilesReturnsImmediateChildren(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		// Generate a random number of subdirectories and files
		numDirs := rapid.IntRange(0, 5).Draw(rt, "numDirs")
		numFiles := rapid.IntRange(0, 5).Draw(rt, "numFiles")

		createdDirs := make(map[string]bool)
		createdFiles := make(map[string]bool)

		for i := 0; i < numDirs; i++ {
			name := folderNameGen().Draw(rt, "dirName")
			lower := strings.ToLower(name)
			if createdDirs[lower] || createdFiles[lower] {
				continue
			}
			if err := os.Mkdir(filepath.Join(dataDir, name), 0755); err != nil {
				continue
			}
			createdDirs[lower] = true
		}

		for i := 0; i < numFiles; i++ {
			name := "note_" + folderNameGen().Draw(rt, "fileName") + ".md"
			lower := strings.ToLower(name)
			if createdDirs[lower] || createdFiles[lower] {
				continue
			}
			if err := os.WriteFile(filepath.Join(dataDir, name), []byte("content"), 0644); err != nil {
				continue
			}
			createdFiles[lower] = true
		}

		// Also create a non-matching file to verify it's filtered out
		os.WriteFile(filepath.Join(dataDir, "users.json"), []byte("{}"), 0644)

		// Also create a nested file inside a subdir to verify non-recursive behavior
		if len(createdDirs) > 0 {
			for dirName := range createdDirs {
				nestedPath := filepath.Join(dataDir, dirName, "nested.txt")
				os.WriteFile(nestedPath, []byte("deep"), 0644)
				break
			}
		}

		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Build expected entries from disk
		diskEntries, err := os.ReadDir(dataDir)
		if err != nil {
			rt.Fatalf("os.ReadDir failed: %v", err)
		}

		var expectedNames []string
		for _, e := range diskEntries {
			name := e.Name()
			if e.IsDir() {
				expectedNames = append(expectedNames, name)
			} else if (strings.HasPrefix(name, "note_") && strings.HasSuffix(name, ".md") && len(name) > len("note_")+len(".md")) ||
				(strings.HasPrefix(name, "tasks_") && strings.HasSuffix(name, ".md") && len(name) > len("tasks_")+len(".md")) {
				expectedNames = append(expectedNames, name)
			}
		}

		sort.Strings(expectedNames)
		gotNames := make([]string, len(resp.Entries))
		for i, entry := range resp.Entries {
			// Extract name from path (path is just name when parent_dir is empty)
			gotNames[i] = entry.Path
		}
		sort.Strings(gotNames)

		// (a) every immediate child represented exactly once
		if len(gotNames) != len(expectedNames) {
			rt.Fatalf("expected %d entries, got %d\nexpected: %v\ngot: %v", len(expectedNames), len(gotNames), expectedNames, gotNames)
		}
		for i := range expectedNames {
			if gotNames[i] != expectedNames[i] {
				rt.Fatalf("entry mismatch at %d: expected %q, got %q", i, expectedNames[i], gotNames[i])
			}
		}

		// (b) directory entries have FOLDER type
		// (c) file entries have NOTE or TASK_LIST type
		for _, entry := range resp.Entries {
			fullPath := filepath.Join(dataDir, entry.Path)
			info, statErr := os.Stat(fullPath)
			if statErr != nil {
				rt.Fatalf("entry %q not found on disk: %v", entry.Path, statErr)
			}
			if info.IsDir() && entry.ItemType != filev1.ItemType_ITEM_TYPE_FOLDER {
				rt.Fatalf("directory entry %q should have FOLDER type, got %v", entry.Path, entry.ItemType)
			}
			if !info.IsDir() && entry.ItemType == filev1.ItemType_ITEM_TYPE_FOLDER {
				rt.Fatalf("file entry %q should not have FOLDER type", entry.Path)
			}
		}

		// (d) no deeper entries included — none should contain a path separator
		for _, entry := range resp.Entries {
			if strings.Contains(entry.Path, "/") || strings.Contains(entry.Path, "\\") {
				rt.Fatalf("entry %q contains path separator — deeper entry leaked", entry.Path)
			}
		}
	})
}

// Feature: list-files-enrichment, Property 2: ListFiles on non-existent path returns NotFound
// Validates: Requirements 7.1, 7.2, 7.3, 7.4
func TestProperty2_ListFilesNonExistentPathNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "nonExistentPath")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		_, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: name,
		})
		if err == nil {
			rt.Fatalf("expected NotFound error for non-existent path %q, got nil", name)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("expected connect.Error, got %T: %v", err, err)
		}
		if connErr.Code() != connect.CodeNotFound {
			rt.Fatalf("expected NotFound, got %v", connErr.Code())
		}
	})
}

// Feature: list-files-enrichment, Property 3: ListFiles on file path returns NotFound
// Validates: Requirements 7.1, 7.2, 7.3, 7.4
func TestProperty3_ListFilesFilePathNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		fileName := folderNameGen().Draw(rt, "fileName")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		// Create a regular file
		filePath := filepath.Join(dataDir, fileName)
		if err := os.WriteFile(filePath, []byte("data"), 0644); err != nil {
			rt.Fatalf("failed to create file: %v", err)
		}

		_, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: fileName,
		})
		if err == nil {
			rt.Fatalf("expected NotFound error for file path %q, got nil", fileName)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("expected connect.Error, got %T: %v", err, err)
		}
		if connErr.Code() != connect.CodeNotFound {
			rt.Fatalf("expected NotFound, got %v", connErr.Code())
		}
		if !strings.Contains(connErr.Message(), "not a directory") {
			rt.Fatalf("expected 'not a directory' in message, got %q", connErr.Message())
		}
	})
}

// Feature: list-files-enrichment, Property 4: ListFiles on path-traversal returns InvalidArgument
// Validates: Requirements 7.1, 7.2, 7.3, 7.4
func TestProperty4_ListFilesPathTraversalInvalidArgument(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate path-traversal strings with ../ sequences
		numSegments := rapid.IntRange(1, 5).Draw(rt, "numSegments")
		segments := make([]string, numSegments)
		for i := range segments {
			segments[i] = ".."
		}
		traversalPath := strings.Join(segments, "/")

		// Optionally append a suffix
		suffix := rapid.SampledFrom([]string{"", "/etc", "/passwd", "/tmp"}).Draw(rt, "suffix")
		traversalPath += suffix

		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		_, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: traversalPath,
		})
		if err == nil {
			rt.Fatalf("expected InvalidArgument error for traversal path %q, got nil", traversalPath)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("expected connect.Error, got %T: %v", err, err)
		}
		if connErr.Code() != connect.CodeInvalidArgument {
			rt.Fatalf("expected InvalidArgument, got %v for path %q", connErr.Code(), traversalPath)
		}
	})
}

// Validates: Requirements 8.1, 8.2
func TestListFiles_EmptyDirectory(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, testDB(t), nopLogger())

	// Create an empty subdirectory
	emptyDir := "emptydir"
	if err := os.Mkdir(filepath.Join(dataDir, emptyDir), 0755); err != nil {
		t.Fatalf("failed to create empty dir: %v", err)
	}

	resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
		ParentDir: emptyDir,
	})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}
	if len(resp.Entries) != 0 {
		t.Fatalf("expected 0 entries for empty directory, got %d: %v", len(resp.Entries), resp.Entries)
	}
}

// Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5
func TestListFiles_RootPath(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, testDB(t), nopLogger())

	// Create some children in the data directory
	os.Mkdir(filepath.Join(dataDir, "folderA"), 0755)
	os.WriteFile(filepath.Join(dataDir, "note_hello.md"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dataDir, "tasks_todo.md"), []byte("todo"), 0644)
	os.WriteFile(filepath.Join(dataDir, "note_hello.txt"), []byte("hello"), 0644)  // wrong suffix, should be filtered
	os.WriteFile(filepath.Join(dataDir, "tasks_todo.txt"), []byte("todo"), 0644)   // wrong suffix, should be filtered
	os.WriteFile(filepath.Join(dataDir, "users.json"), []byte("{}"), 0644)         // no matching prefix, should be filtered

	resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
		ParentDir: "",
	})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	entries := make(map[string]*filev1.FileEntry)
	for _, e := range resp.Entries {
		entries[e.Path] = e
	}

	if _, ok := entries["folderA"]; !ok {
		t.Fatal("expected 'folderA' in entries")
	}
	if entries["folderA"].ItemType != filev1.ItemType_ITEM_TYPE_FOLDER {
		t.Fatalf("expected folderA to have FOLDER type, got %v", entries["folderA"].ItemType)
	}
	
	if _, ok := entries["note_hello.md"]; !ok {
		t.Fatal("expected 'note_hello.md' in entries")
	}
	if entries["note_hello.md"].ItemType != filev1.ItemType_ITEM_TYPE_NOTE {
		t.Fatalf("expected note_hello.md to have NOTE type, got %v", entries["note_hello.md"].ItemType)
	}
	
	if _, ok := entries["tasks_todo.md"]; !ok {
		t.Fatal("expected 'tasks_todo.md' in entries")
	}
	if entries["tasks_todo.md"].ItemType != filev1.ItemType_ITEM_TYPE_TASK_LIST {
		t.Fatalf("expected tasks_todo.md to have TASK_LIST type, got %v", entries["tasks_todo.md"].ItemType)
	}
	
	if _, ok := entries["note_hello.txt"]; ok {
		t.Fatal("'note_hello.txt' should be filtered out (wrong suffix)")
	}
	if _, ok := entries["tasks_todo.txt"]; ok {
		t.Fatal("'tasks_todo.txt' should be filtered out (wrong suffix)")
	}
	if _, ok := entries["users.json"]; ok {
		t.Fatal("'users.json' should be filtered out")
	}
	if len(resp.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(resp.Entries), resp.Entries)
	}
}

func TestListFiles_RootSlashRejected(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, testDB(t), nopLogger())

	_, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
		ParentDir: "/",
	})
	if err == nil {
		t.Fatal("expected error for leading-slash parent_dir, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestListFiles_EmptyParentDirListsRoot(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, testDB(t), nopLogger())

	os.Mkdir(filepath.Join(dataDir, "folderA"), 0755)
	os.WriteFile(filepath.Join(dataDir, "note_hello.md"), []byte("hi"), 0644)
	os.WriteFile(filepath.Join(dataDir, "tasks_todo.md"), []byte("todo"), 0644)

	resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
		ParentDir: "",
	})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	expected := map[string]bool{
		"folderA":       true,
		"note_hello.md": true,
		"tasks_todo.md": true,
	}
	for _, e := range resp.Entries {
		if !expected[e.Path] {
			t.Errorf("unexpected path: %q", e.Path)
		}
		delete(expected, e.Path)
	}
	for p := range expected {
		t.Errorf("missing expected path: %q", p)
	}
}

