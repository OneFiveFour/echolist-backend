package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	filev1 "echolist-backend/proto/gen/file/v1"
	"pgregory.net/rapid"
)

// Feature: list-files-enrichment, Property 1: Classification and filtering correctness
// Validates: Requirements 2.2, 2.3, 2.4, 2.5
func TestProperty1_ClassificationAndFiltering(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		// Generate random mix of subdirectories, note files, task files, and unrecognized files
		numDirs := rapid.IntRange(0, 5).Draw(rt, "numDirs")
		numNotes := rapid.IntRange(0, 5).Draw(rt, "numNotes")
		numTasks := rapid.IntRange(0, 5).Draw(rt, "numTasks")
		numUnrecognized := rapid.IntRange(0, 3).Draw(rt, "numUnrecognized")

		createdDirs := make(map[string]bool)
		createdNotes := make(map[string]bool)
		createdTasks := make(map[string]bool)

		// Create directories
		for i := 0; i < numDirs; i++ {
			name := folderNameGen().Draw(rt, "dirName")
			if createdDirs[name] {
				continue
			}
			if err := os.Mkdir(filepath.Join(dataDir, name), 0755); err != nil {
				continue
			}
			createdDirs[name] = true
		}

		// Create note files
		for i := 0; i < numNotes; i++ {
			name := "note_" + folderNameGen().Draw(rt, "noteName") + ".md"
			if createdNotes[name] {
				continue
			}
			if err := os.WriteFile(filepath.Join(dataDir, name), []byte("note content"), 0644); err != nil {
				continue
			}
			createdNotes[name] = true
		}

		// Create task files
		for i := 0; i < numTasks; i++ {
			name := "tasks_" + folderNameGen().Draw(rt, "taskName") + ".md"
			if createdTasks[name] {
				continue
			}
			if err := os.WriteFile(filepath.Join(dataDir, name), []byte("- [ ] Task 1\n"), 0644); err != nil {
				continue
			}
			createdTasks[name] = true
		}

		// Create unrecognized files
		for i := 0; i < numUnrecognized; i++ {
			name := "data_" + folderNameGen().Draw(rt, "unrecognizedName") + ".json"
			os.WriteFile(filepath.Join(dataDir, name), []byte("{}"), 0644)
		}

		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Verify entry count matches expected (dirs + notes + tasks, no unrecognized)
		expectedCount := len(createdDirs) + len(createdNotes) + len(createdTasks)
		if len(resp.Entries) != expectedCount {
			rt.Fatalf("expected %d entries, got %d", expectedCount, len(resp.Entries))
		}

		// Verify each entry has correct item_type
		for _, entry := range resp.Entries {
			if createdDirs[entry.Path] {
				if entry.ItemType != filev1.ItemType_ITEM_TYPE_FOLDER {
					rt.Fatalf("directory %q should have FOLDER type, got %v", entry.Path, entry.ItemType)
				}
			} else if createdNotes[entry.Path] {
				if entry.ItemType != filev1.ItemType_ITEM_TYPE_NOTE {
					rt.Fatalf("note %q should have NOTE type, got %v", entry.Path, entry.ItemType)
				}
			} else if createdTasks[entry.Path] {
				if entry.ItemType != filev1.ItemType_ITEM_TYPE_TASK_LIST {
					rt.Fatalf("task list %q should have TASK_LIST type, got %v", entry.Path, entry.ItemType)
				}
			} else {
				rt.Fatalf("unexpected entry %q in response", entry.Path)
			}
		}

		// Verify no unrecognized files in response
		for _, entry := range resp.Entries {
			if strings.HasPrefix(entry.Path, "data_") {
				rt.Fatalf("unrecognized file %q should be filtered out", entry.Path)
			}
		}
	})
}

// Feature: list-files-enrichment, Property 2: Path construction correctness
// Validates: Requirements 6.1, 6.2
func TestProperty2_PathConstruction(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		// Create a subdirectory with some files
		subDir := folderNameGen().Draw(rt, "subDir")
		subDirPath := filepath.Join(dataDir, subDir)
		if err := os.Mkdir(subDirPath, 0755); err != nil {
			rt.Skipf("failed to create subdir: %v", err)
		}

		// Create files in root and subdir
		os.WriteFile(filepath.Join(dataDir, "note_root.md"), []byte("root"), 0644)
		os.WriteFile(filepath.Join(subDirPath, "note_sub.md"), []byte("sub"), 0644)

		// Test empty parent_dir
		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListFiles with empty parent_dir failed: %v", err)
		}

		for _, entry := range resp.Entries {
			// Path should be just the name (no leading separator)
			if strings.HasPrefix(entry.Path, "/") {
				rt.Fatalf("path %q should not have leading separator when parent_dir is empty", entry.Path)
			}
			// Path should not contain separator (immediate children only)
			if strings.Contains(entry.Path, "/") {
				rt.Fatalf("path %q should not contain separator when parent_dir is empty", entry.Path)
			}
		}

		// Test non-empty parent_dir
		resp, err = srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: subDir,
		})
		if err != nil {
			rt.Fatalf("ListFiles with parent_dir %q failed: %v", subDir, err)
		}

		for _, entry := range resp.Entries {
			// Path should be parent_dir + "/" + name
			expectedPrefix := subDir + "/"
			if !strings.HasPrefix(entry.Path, expectedPrefix) {
				rt.Fatalf("path %q should start with %q", entry.Path, expectedPrefix)
			}
		}
	})
}

// Feature: list-files-enrichment, Property 3: Folder metadata correctness
// Validates: Requirements 3.1, 3.2, 3.3
func TestProperty3_FolderMetadata(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		// Create a folder with random children
		folderName := folderNameGen().Draw(rt, "folderName")
		folderPath := filepath.Join(dataDir, folderName)
		if err := os.Mkdir(folderPath, 0755); err != nil {
			rt.Skipf("failed to create folder: %v", err)
		}

		// Add random mix of children
		numSubdirs := rapid.IntRange(0, 3).Draw(rt, "numSubdirs")
		numNotes := rapid.IntRange(0, 3).Draw(rt, "numNotes")
		numTasks := rapid.IntRange(0, 3).Draw(rt, "numTasks")
		numUnrecognized := rapid.IntRange(0, 2).Draw(rt, "numUnrecognized")

		expectedChildCount := numSubdirs + numNotes + numTasks

		for i := 0; i < numSubdirs; i++ {
			os.Mkdir(filepath.Join(folderPath, "sub"+string(rune('A'+i))), 0755)
		}
		for i := 0; i < numNotes; i++ {
			os.WriteFile(filepath.Join(folderPath, "note_"+string(rune('A'+i))+".md"), []byte("note"), 0644)
		}
		for i := 0; i < numTasks; i++ {
			os.WriteFile(filepath.Join(folderPath, "tasks_"+string(rune('A'+i))+".md"), []byte("tasks"), 0644)
		}
		for i := 0; i < numUnrecognized; i++ {
			os.WriteFile(filepath.Join(folderPath, "data"+string(rune('A'+i))+".json"), []byte("{}"), 0644)
		}

		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Find the folder entry
		var folderEntry *filev1.FileEntry
		for _, entry := range resp.Entries {
			if entry.Path == folderName && entry.ItemType == filev1.ItemType_ITEM_TYPE_FOLDER {
				folderEntry = entry
				break
			}
		}

		if folderEntry == nil {
			rt.Fatalf("folder %q not found in response", folderName)
		}

		// Verify title equals directory name
		if folderEntry.Title != folderName {
			rt.Fatalf("expected title %q, got %q", folderName, folderEntry.Title)
		}

		// Verify child_count matches expected (excludes unrecognized files)
		folderMeta := folderEntry.GetFolderMetadata()
		if folderMeta == nil {
			rt.Fatal("expected FolderMetadata, got nil")
		}
		if folderMeta.ChildCount != int32(expectedChildCount) {
			rt.Fatalf("expected child_count %d, got %d", expectedChildCount, folderMeta.ChildCount)
		}
	})
}

// Feature: list-files-enrichment, Property 4: Note metadata correctness
// Validates: Requirements 4.1, 4.2, 4.3, 4.4
func TestProperty4_NoteMetadata(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		// Generate random note content
		contentLen := rapid.IntRange(0, 250).Draw(rt, "contentLen")
		content := rapid.StringN(contentLen, contentLen, -1).Draw(rt, "content")

		noteName := "note_Test.md"
		if err := os.WriteFile(filepath.Join(dataDir, noteName), []byte(content), 0644); err != nil {
			rt.Skipf("failed to create note: %v", err)
		}

		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Find the note entry
		var noteEntry *filev1.FileEntry
		for _, entry := range resp.Entries {
			if entry.Path == noteName {
				noteEntry = entry
				break
			}
		}

		if noteEntry == nil {
			rt.Fatal("note not found in response")
		}

		// Verify title extraction
		if noteEntry.Title != "Test" {
			rt.Fatalf("expected title %q, got %q", "Test", noteEntry.Title)
		}

		// Verify updated_at is non-zero
		noteMeta := noteEntry.GetNoteMetadata()
		if noteMeta == nil {
			rt.Fatal("expected NoteMetadata, got nil")
		}
		if noteMeta.UpdatedAt == 0 {
			rt.Error("expected non-zero updated_at")
		}

		// Verify preview is first 100 characters (rune-safe)
		contentRunes := []rune(content)
		var expectedPreview string
		if len(contentRunes) > 100 {
			expectedPreview = string(contentRunes[:100])
		} else {
			expectedPreview = content
		}

		if noteMeta.Preview != expectedPreview {
			rt.Fatalf("preview mismatch: expected %d runes, got %d runes", len([]rune(expectedPreview)), len([]rune(noteMeta.Preview)))
		}
	})
}

// Feature: list-files-enrichment, Property 5: Task list metadata correctness
// Validates: Requirements 5.1, 5.2, 5.3, 5.4
func TestProperty5_TaskListMetadata(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		// Generate random task list
		numTasks := rapid.IntRange(0, 10).Draw(rt, "numTasks")
		expectedDone := 0

		var lines []string
		for i := 0; i < numTasks; i++ {
			done := rapid.Bool().Draw(rt, "done")
			checkbox := "[ ]"
			if done {
				checkbox = "[x]"
				expectedDone++
			}
			lines = append(lines, "- "+checkbox+" Task "+string(rune('A'+i)))
		}

		var content []byte
		if len(lines) > 0 {
			content = []byte(strings.Join(lines, "\n"))
		}
		taskName := "tasks_Test.md"
		if err := os.WriteFile(filepath.Join(dataDir, taskName), content, 0644); err != nil {
			rt.Skipf("failed to create task file: %v", err)
		}

		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Find the task list entry
		var taskEntry *filev1.FileEntry
		for _, entry := range resp.Entries {
			if entry.Path == taskName {
				taskEntry = entry
				break
			}
		}

		if taskEntry == nil {
			rt.Fatal("task list not found in response")
		}

		// Verify title extraction
		if taskEntry.Title != "Test" {
			rt.Fatalf("expected title %q, got %q", "Test", taskEntry.Title)
		}

		// Verify updated_at is non-zero
		taskMeta := taskEntry.GetTaskListMetadata()
		if taskMeta == nil {
			rt.Fatal("expected TaskListMetadata, got nil")
		}
		if taskMeta.UpdatedAt == 0 {
			rt.Error("expected non-zero updated_at")
		}

		// Verify task counts
		if taskMeta.TotalTaskCount != int32(numTasks) {
			rt.Fatalf("expected total_task_count %d, got %d", numTasks, taskMeta.TotalTaskCount)
		}
		if taskMeta.DoneTaskCount != int32(expectedDone) {
			rt.Fatalf("expected done_task_count %d, got %d", expectedDone, taskMeta.DoneTaskCount)
		}
	})
}

// Feature: list-files-enrichment, Property 6: Path round-trip usability
// Validates: Requirements 6.3
func TestProperty6_PathRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		// Create a subdirectory with files
		subDir := folderNameGen().Draw(rt, "subDir")
		subDirPath := filepath.Join(dataDir, subDir)
		if err := os.Mkdir(subDirPath, 0755); err != nil {
			rt.Skipf("failed to create subdir: %v", err)
		}

		os.WriteFile(filepath.Join(subDirPath, "note_Test.md"), []byte("content"), 0644)
		os.WriteFile(filepath.Join(subDirPath, "tasks_Todo.md"), []byte("- [ ] Task\n"), 0644)

		// List the subdirectory
		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: subDir,
		})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Try to use each returned path in subsequent calls
		for _, entry := range resp.Entries {
			switch entry.ItemType {
			case filev1.ItemType_ITEM_TYPE_FOLDER:
				// Should be usable as parent_dir in ListFiles
				_, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
					ParentDir: entry.Path,
				})
				if err != nil {
					rt.Fatalf("ListFiles with path %q failed: %v", entry.Path, err)
				}

			case filev1.ItemType_ITEM_TYPE_NOTE:
				// Path should be usable in GetNote (if we had access to notes service)
				// For now, just verify the path format is correct
				if !strings.HasSuffix(entry.Path, ".md") {
					rt.Fatalf("note path %q should end with .md", entry.Path)
				}
				if !strings.Contains(entry.Path, "/") {
					rt.Fatalf("note path %q should contain parent dir", entry.Path)
				}

			case filev1.ItemType_ITEM_TYPE_TASK_LIST:
				// Path should be usable in GetTaskList (if we had access to tasks service)
				// For now, just verify the path format is correct
				if !strings.HasSuffix(entry.Path, ".md") {
					rt.Fatalf("task list path %q should end with .md", entry.Path)
				}
				if !strings.Contains(entry.Path, "/") {
					rt.Fatalf("task list path %q should contain parent dir", entry.Path)
				}
			}
		}
	})
}

// Feature: list-files-enrichment, Property 7: Non-recursive listing
// Validates: Requirements 9.1, 9.2
func TestProperty7_NonRecursive(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := NewFileServer(dataDir, testDB(t), nopLogger())

		// Create nested directory structure
		depth := rapid.IntRange(1, 4).Draw(rt, "depth")
		currentPath := dataDir
		var pathSegments []string

		for i := 0; i < depth; i++ {
			segment := folderNameGen().Draw(rt, "segment")
			pathSegments = append(pathSegments, segment)
			currentPath = filepath.Join(currentPath, segment)
			if err := os.Mkdir(currentPath, 0755); err != nil {
				rt.Skipf("failed to create nested dir: %v", err)
			}
		}

		// Create a file in the deepest directory
		os.WriteFile(filepath.Join(currentPath, "note_Deep.md"), []byte("deep"), 0644)

		// List from root
		resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Verify no entry contains path separator (all immediate children)
		for _, entry := range resp.Entries {
			if strings.Contains(entry.Path, "/") || strings.Contains(entry.Path, "\\") {
				rt.Fatalf("entry %q contains path separator — deeper entry leaked", entry.Path)
			}
		}

		// List from first level
		if len(pathSegments) > 0 {
			resp, err = srv.ListFiles(context.Background(), &filev1.ListFilesRequest{
				ParentDir: pathSegments[0],
			})
			if err != nil {
				rt.Fatalf("ListFiles with parent_dir failed: %v", err)
			}

			// Verify paths are parent_dir + "/" + name (no deeper nesting)
			for _, entry := range resp.Entries {
				expectedPrefix := pathSegments[0] + "/"
				if !strings.HasPrefix(entry.Path, expectedPrefix) {
					rt.Fatalf("path %q should start with %q", entry.Path, expectedPrefix)
				}
				// Remove prefix and check no more separators
				remainder := strings.TrimPrefix(entry.Path, expectedPrefix)
				if strings.Contains(remainder, "/") || strings.Contains(remainder, "\\") {
					rt.Fatalf("path %q contains nested separator after prefix", entry.Path)
				}
			}
		}
	})
}
