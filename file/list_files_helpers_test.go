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
	srv := NewFileServer(dataDir, testDB(t), nopLogger())

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
