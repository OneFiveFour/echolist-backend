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

// Feature: file-service-refactor, Property 1: ListFiles returns immediate children with correct entry format
// Validates: Requirements 3.1, 3.2, 3.3, 3.4
func TestProperty1_ListFilesReturnsImmediateChildren(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir)

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
			name := "note_" + folderNameGen().Draw(rt, "fileName")
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

		var expected []string
		for _, e := range diskEntries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			} else if !strings.HasPrefix(name, "note_") && !strings.HasPrefix(name, "tasks_") {
				continue
			}
			expected = append(expected, name)
		}

		sort.Strings(expected)
		got := make([]string, len(resp.Entries))
		copy(got, resp.Entries)
		sort.Strings(got)

		// (a) every immediate child represented exactly once
		if len(got) != len(expected) {
			rt.Fatalf("expected %d entries, got %d\nexpected: %v\ngot: %v", len(expected), len(got), expected, got)
		}
		for i := range expected {
			if got[i] != expected[i] {
				rt.Fatalf("entry mismatch at %d: expected %q, got %q", i, expected[i], got[i])
			}
		}

		// (b) directory entries end with /
		// (c) file entries do not end with /
		for _, entry := range resp.Entries {
			nameWithoutSlash := strings.TrimSuffix(entry, "/")
			fullPath := filepath.Join(dataDir, nameWithoutSlash)
			info, statErr := os.Stat(fullPath)
			if statErr != nil {
				rt.Fatalf("entry %q not found on disk: %v", entry, statErr)
			}
			if info.IsDir() && !strings.HasSuffix(entry, "/") {
				rt.Fatalf("directory entry %q should end with /", entry)
			}
			if !info.IsDir() && strings.HasSuffix(entry, "/") {
				rt.Fatalf("file entry %q should not end with /", entry)
			}
		}

		// (d) no deeper entries included — none should contain a path separator
		for _, entry := range resp.Entries {
			trimmed := strings.TrimSuffix(entry, "/")
			if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
				rt.Fatalf("entry %q contains path separator — deeper entry leaked", entry)
			}
		}
	})
}

// Feature: file-service-refactor, Property 2: ListFiles on non-existent path returns NotFound
// Validates: Requirements 3.5
func TestProperty2_ListFilesNonExistentPathNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		name := folderNameGen().Draw(rt, "nonExistentPath")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir)

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

// Feature: file-service-refactor, Property 3: ListFiles on file path returns NotFound
// Validates: Requirements 3.6
func TestProperty3_ListFilesFilePathNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		fileName := folderNameGen().Draw(rt, "fileName")
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir)

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

// Feature: file-service-refactor, Property 4: ListFiles on path-traversal returns InvalidArgument
// Validates: Requirements 3.7
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
		srv := NewFileServer(dataDir)

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

// Validates: Requirements 3.1, 3.4
func TestListFiles_EmptyDirectory(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir)

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

// Validates: Requirements 3.1, 3.4
func TestListFiles_RootPath(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir)

	// Create some children in the data directory
	os.Mkdir(filepath.Join(dataDir, "folderA"), 0755)
	os.WriteFile(filepath.Join(dataDir, "note_hello.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dataDir, "tasks_todo.txt"), []byte("todo"), 0644)
	os.WriteFile(filepath.Join(dataDir, "users.json"), []byte("{}"), 0644) // should be filtered

	resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
		ParentDir: "",
	})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	entries := make(map[string]bool)
	for _, e := range resp.Entries {
		entries[e] = true
	}

	if !entries["folderA/"] {
		t.Fatal("expected 'folderA/' in entries")
	}
	if !entries["note_hello.txt"] {
		t.Fatal("expected 'note_hello.txt' in entries")
	}
	if !entries["tasks_todo.txt"] {
		t.Fatal("expected 'tasks_todo.txt' in entries")
	}
	if entries["users.json"] {
		t.Fatal("'users.json' should be filtered out")
	}
	if len(resp.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(resp.Entries), resp.Entries)
	}
}
