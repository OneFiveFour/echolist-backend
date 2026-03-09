package file

import (
	"os"
	"path/filepath"
	"testing"

	filev1 "echolist-backend/proto/gen/file/v1"
)

// Test entryPath helper function
func TestEntryPath(t *testing.T) {
	tests := []struct {
		name             string
		requestParentDir string
		entryName        string
		want             string
	}{
		{"empty parent dir", "", "file.md", "file.md"},
		{"empty parent dir with folder", "", "folder", "folder"},
		{"non-empty parent dir", "Work", "note_Meeting.md", "Work/note_Meeting.md"},
		{"non-empty parent dir with folder", "Work", "2026", "Work/2026"},
		{"nested parent dir", "Work/2026", "note_Test.md", "Work/2026/note_Test.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := entryPath(tt.requestParentDir, tt.entryName)
			if got != tt.want {
				t.Errorf("entryPath(%q, %q) = %q, want %q", tt.requestParentDir, tt.entryName, got, tt.want)
			}
		})
	}
}

// Test buildFolderEntry with various child combinations
func TestBuildFolderEntry(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, nopLogger())

	tests := []struct {
		name             string
		setupFunc        func(string) string // returns absPath of the folder
		requestParentDir string
		folderName       string
		wantChildCount   int32
	}{
		{
			name: "empty folder",
			setupFunc: func(base string) string {
				folderPath := filepath.Join(base, "empty")
				os.Mkdir(folderPath, 0755)
				return folderPath
			},
			requestParentDir: "",
			folderName:       "empty",
			wantChildCount:   0,
		},
		{
			name: "folder with only subdirectories",
			setupFunc: func(base string) string {
				folderPath := filepath.Join(base, "dirs")
				os.Mkdir(folderPath, 0755)
				os.Mkdir(filepath.Join(folderPath, "sub1"), 0755)
				os.Mkdir(filepath.Join(folderPath, "sub2"), 0755)
				return folderPath
			},
			requestParentDir: "",
			folderName:       "dirs",
			wantChildCount:   2,
		},
		{
			name: "folder with only notes",
			setupFunc: func(base string) string {
				folderPath := filepath.Join(base, "notes")
				os.Mkdir(folderPath, 0755)
				os.WriteFile(filepath.Join(folderPath, "note_A.md"), []byte("a"), 0644)
				os.WriteFile(filepath.Join(folderPath, "note_B.md"), []byte("b"), 0644)
				return folderPath
			},
			requestParentDir: "",
			folderName:       "notes",
			wantChildCount:   2,
		},
		{
			name: "folder with only task lists",
			setupFunc: func(base string) string {
				folderPath := filepath.Join(base, "tasks")
				os.Mkdir(folderPath, 0755)
				os.WriteFile(filepath.Join(folderPath, "tasks_Sprint.md"), []byte("tasks"), 0644)
				return folderPath
			},
			requestParentDir: "",
			folderName:       "tasks",
			wantChildCount:   1,
		},
		{
			name: "folder with mixed recognized children",
			setupFunc: func(base string) string {
				folderPath := filepath.Join(base, "mixed")
				os.Mkdir(folderPath, 0755)
				os.Mkdir(filepath.Join(folderPath, "subfolder"), 0755)
				os.WriteFile(filepath.Join(folderPath, "note_Meeting.md"), []byte("note"), 0644)
				os.WriteFile(filepath.Join(folderPath, "tasks_Todo.md"), []byte("tasks"), 0644)
				return folderPath
			},
			requestParentDir: "",
			folderName:       "mixed",
			wantChildCount:   3,
		},
		{
			name: "folder with unrecognized files only",
			setupFunc: func(base string) string {
				folderPath := filepath.Join(base, "unrecognized")
				os.Mkdir(folderPath, 0755)
				os.WriteFile(filepath.Join(folderPath, "data.json"), []byte("{}"), 0644)
				os.WriteFile(filepath.Join(folderPath, "config.txt"), []byte("config"), 0644)
				return folderPath
			},
			requestParentDir: "",
			folderName:       "unrecognized",
			wantChildCount:   0,
		},
		{
			name: "folder with mixed recognized and unrecognized",
			setupFunc: func(base string) string {
				folderPath := filepath.Join(base, "mixedWithUnrecognized")
				os.Mkdir(folderPath, 0755)
				os.Mkdir(filepath.Join(folderPath, "subfolder"), 0755)
				os.WriteFile(filepath.Join(folderPath, "note_A.md"), []byte("note"), 0644)
				os.WriteFile(filepath.Join(folderPath, "data.json"), []byte("{}"), 0644)
				return folderPath
			},
			requestParentDir: "",
			folderName:       "mixedWithUnrecognized",
			wantChildCount:   2, // only subfolder and note_A.md
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath := tt.setupFunc(dataDir)
			entry := srv.buildFolderEntry(absPath, tt.folderName, tt.requestParentDir)

			if entry.ItemType != filev1.ItemType_ITEM_TYPE_FOLDER {
				t.Errorf("expected ItemType FOLDER, got %v", entry.ItemType)
			}
			if entry.Title != tt.folderName {
				t.Errorf("expected title %q, got %q", tt.folderName, entry.Title)
			}
			if entry.Path != entryPath(tt.requestParentDir, tt.folderName) {
				t.Errorf("expected path %q, got %q", entryPath(tt.requestParentDir, tt.folderName), entry.Path)
			}

			folderMeta := entry.GetFolderMetadata()
			if folderMeta == nil {
				t.Fatal("expected FolderMetadata, got nil")
			}
			if folderMeta.ChildCount != tt.wantChildCount {
				t.Errorf("expected child_count %d, got %d", tt.wantChildCount, folderMeta.ChildCount)
			}
		})
	}
}

// Test buildNoteEntry with different content lengths and UTF-8
func TestBuildNoteEntry(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, nopLogger())

	tests := []struct {
		name             string
		fileName         string
		content          string
		requestParentDir string
		wantTitle        string
		wantPreviewLen   int // length in runes
	}{
		{
			name:             "note with short content",
			fileName:         "note_Short.md",
			content:          "Hello",
			requestParentDir: "",
			wantTitle:        "Short",
			wantPreviewLen:   5,
		},
		{
			name:             "note with exactly 100 characters",
			fileName:         "note_Exact100.md",
			content:          string(make([]byte, 100)), // 100 bytes of zeros
			requestParentDir: "",
			wantTitle:        "Exact100",
			wantPreviewLen:   100,
		},
		{
			name:             "note with 101 characters",
			fileName:         "note_Over100.md",
			content:          string(make([]byte, 101)),
			requestParentDir: "",
			wantTitle:        "Over100",
			wantPreviewLen:   100,
		},
		{
			name:             "note with multi-byte UTF-8 content",
			fileName:         "note_UTF8.md",
			content:          "Hello 世界 " + string(make([]byte, 100)), // multi-byte chars + padding
			requestParentDir: "",
			wantTitle:        "UTF8",
			wantPreviewLen:   100,
		},
		{
			name:             "note with non-empty parent dir",
			fileName:         "note_Nested.md",
			content:          "Nested content",
			requestParentDir: "Work",
			wantTitle:        "Nested",
			wantPreviewLen:   14,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath := filepath.Join(dataDir, tt.fileName)
			if err := os.WriteFile(absPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create note file: %v", err)
			}

			entry := srv.buildNoteEntry(absPath, tt.fileName, tt.requestParentDir)

			if entry.ItemType != filev1.ItemType_ITEM_TYPE_NOTE {
				t.Errorf("expected ItemType NOTE, got %v", entry.ItemType)
			}
			if entry.Title != tt.wantTitle {
				t.Errorf("expected title %q, got %q", tt.wantTitle, entry.Title)
			}
			if entry.Path != entryPath(tt.requestParentDir, tt.fileName) {
				t.Errorf("expected path %q, got %q", entryPath(tt.requestParentDir, tt.fileName), entry.Path)
			}

			noteMeta := entry.GetNoteMetadata()
			if noteMeta == nil {
				t.Fatal("expected NoteMetadata, got nil")
			}
			if noteMeta.UpdatedAt == 0 {
				t.Error("expected non-zero updated_at")
			}

			previewRunes := []rune(noteMeta.Preview)
			if len(previewRunes) != tt.wantPreviewLen {
				t.Errorf("expected preview length %d runes, got %d", tt.wantPreviewLen, len(previewRunes))
			}
		})
	}
}

// Test buildNoteEntry error handling
func TestBuildNoteEntry_ErrorHandling(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, nopLogger())

	t.Run("non-existent file", func(t *testing.T) {
		absPath := filepath.Join(dataDir, "note_Missing.md")
		entry := srv.buildNoteEntry(absPath, "note_Missing.md", "")

		// Should still return an entry with zero/empty values
		if entry.ItemType != filev1.ItemType_ITEM_TYPE_NOTE {
			t.Errorf("expected ItemType NOTE, got %v", entry.ItemType)
		}
		noteMeta := entry.GetNoteMetadata()
		if noteMeta == nil {
			t.Fatal("expected NoteMetadata, got nil")
		}
		if noteMeta.UpdatedAt != 0 {
			t.Errorf("expected zero updated_at for missing file, got %d", noteMeta.UpdatedAt)
		}
		if noteMeta.Preview != "" {
			t.Errorf("expected empty preview for missing file, got %q", noteMeta.Preview)
		}
	})
}

// Test buildTaskListEntry with various task counts
func TestBuildTaskListEntry(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, nopLogger())

	tests := []struct {
		name             string
		fileName         string
		content          string
		requestParentDir string
		wantTitle        string
		wantTotalCount   int32
		wantDoneCount    int32
	}{
		{
			name:             "empty task list",
			fileName:         "tasks_Empty.md",
			content:          "",
			requestParentDir: "",
			wantTitle:        "Empty",
			wantTotalCount:   0,
			wantDoneCount:    0,
		},
		{
			name:     "task list with one incomplete task",
			fileName: "tasks_OneIncomplete.md",
			content: `- [ ] Task 1
`,
			requestParentDir: "",
			wantTitle:        "OneIncomplete",
			wantTotalCount:   1,
			wantDoneCount:    0,
		},
		{
			name:     "task list with one complete task",
			fileName: "tasks_OneComplete.md",
			content: `- [x] Task 1
`,
			requestParentDir: "",
			wantTitle:        "OneComplete",
			wantTotalCount:   1,
			wantDoneCount:    1,
		},
		{
			name:     "task list with mixed tasks",
			fileName: "tasks_Mixed.md",
			content: `- [x] Task 1
- [ ] Task 2
- [x] Task 3
- [ ] Task 4
`,
			requestParentDir: "",
			wantTitle:        "Mixed",
			wantTotalCount:   4,
			wantDoneCount:    2,
		},
		{
			name:     "task list with non-empty parent dir",
			fileName: "tasks_Nested.md",
			content: `- [x] Done task
- [ ] Todo task
`,
			requestParentDir: "Work",
			wantTitle:        "Nested",
			wantTotalCount:   2,
			wantDoneCount:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath := filepath.Join(dataDir, tt.fileName)
			if err := os.WriteFile(absPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to create task file: %v", err)
			}

			entry := srv.buildTaskListEntry(absPath, tt.fileName, tt.requestParentDir)

			if entry.ItemType != filev1.ItemType_ITEM_TYPE_TASK_LIST {
				t.Errorf("expected ItemType TASK_LIST, got %v", entry.ItemType)
			}
			if entry.Title != tt.wantTitle {
				t.Errorf("expected title %q, got %q", tt.wantTitle, entry.Title)
			}
			if entry.Path != entryPath(tt.requestParentDir, tt.fileName) {
				t.Errorf("expected path %q, got %q", entryPath(tt.requestParentDir, tt.fileName), entry.Path)
			}

			taskMeta := entry.GetTaskListMetadata()
			if taskMeta == nil {
				t.Fatal("expected TaskListMetadata, got nil")
			}
			if taskMeta.UpdatedAt == 0 {
				t.Error("expected non-zero updated_at")
			}
			if taskMeta.TotalTaskCount != tt.wantTotalCount {
				t.Errorf("expected total_task_count %d, got %d", tt.wantTotalCount, taskMeta.TotalTaskCount)
			}
			if taskMeta.DoneTaskCount != tt.wantDoneCount {
				t.Errorf("expected done_task_count %d, got %d", tt.wantDoneCount, taskMeta.DoneTaskCount)
			}
		})
	}
}

// Test buildTaskListEntry error handling
func TestBuildTaskListEntry_ErrorHandling(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewFileServer(dataDir, nopLogger())

	t.Run("non-existent file", func(t *testing.T) {
		absPath := filepath.Join(dataDir, "tasks_Missing.md")
		entry := srv.buildTaskListEntry(absPath, "tasks_Missing.md", "")

		// Should still return an entry with zero values
		if entry.ItemType != filev1.ItemType_ITEM_TYPE_TASK_LIST {
			t.Errorf("expected ItemType TASK_LIST, got %v", entry.ItemType)
		}
		taskMeta := entry.GetTaskListMetadata()
		if taskMeta == nil {
			t.Fatal("expected TaskListMetadata, got nil")
		}
		if taskMeta.UpdatedAt != 0 {
			t.Errorf("expected zero updated_at for missing file, got %d", taskMeta.UpdatedAt)
		}
		if taskMeta.TotalTaskCount != 0 {
			t.Errorf("expected zero total_task_count for missing file, got %d", taskMeta.TotalTaskCount)
		}
		if taskMeta.DoneTaskCount != 0 {
			t.Errorf("expected zero done_task_count for missing file, got %d", taskMeta.DoneTaskCount)
		}
	})

	t.Run("invalid task file content", func(t *testing.T) {
		absPath := filepath.Join(dataDir, "tasks_Invalid.md")
		// Write content that doesn't parse as valid task format
		if err := os.WriteFile(absPath, []byte("not a valid task format\nrandom text"), 0644); err != nil {
			t.Fatalf("failed to create invalid task file: %v", err)
		}

		entry := srv.buildTaskListEntry(absPath, "tasks_Invalid.md", "")

		// Should still return an entry with zero counts
		taskMeta := entry.GetTaskListMetadata()
		if taskMeta == nil {
			t.Fatal("expected TaskListMetadata, got nil")
		}
		// Parser might return empty list or error, either way counts should be 0
		if taskMeta.TotalTaskCount != 0 {
			t.Errorf("expected zero total_task_count for invalid file, got %d", taskMeta.TotalTaskCount)
		}
		if taskMeta.DoneTaskCount != 0 {
			t.Errorf("expected zero done_task_count for invalid file, got %d", taskMeta.DoneTaskCount)
		}
	})
}
