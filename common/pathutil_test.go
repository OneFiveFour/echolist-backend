package common

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSubPath(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		resolved string
		want     bool
	}{
		{"direct child", "/data", "/data/file.txt", true},
		{"nested child", "/data", "/data/a/b/c", true},
		{"same path", "/data", "/data", false},
		{"simple dotdot", "/data", "/data/..", false},
		{"dotdot prefix", "/data", "/etc/passwd", false},
		{"mid-path dotdot escape", "/data", "/data/sub/../../etc", false},
		{"deep mid-path escape", "/data", "/data/a/b/../../../etc", false},
		{"url-encoded not real traversal", "/data", "/data/..%2f..%2f", true},
		{"trailing slash child", "/data/", "/data/file", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSubPath(tt.base, tt.resolved)
			if got != tt.want {
				t.Errorf("IsSubPath(%q, %q) = %v, want %v", tt.base, tt.resolved, got, tt.want)
			}
		})
	}
}

func TestValidatePath_LegitPaths(t *testing.T) {
	dataDir := t.TempDir()

	sub := filepath.Join(dataDir, "notes")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		rel  string
	}{
		{"simple file", "notes/hello.md"},
		{"nested existing dir", "notes"},
		{"non-existent file in existing dir", "notes/new.md"},
		{"non-existent nested path", "notes/a/b/c.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidatePath(dataDir, tt.rel)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			expected := filepath.Join(dataDir, tt.rel)
			if got != expected {
				t.Errorf("got %q, want %q", got, expected)
			}
		})
	}
}

func TestValidatePath_TraversalRejected(t *testing.T) {
	dataDir := t.TempDir()

	traversals := []string{
		"../etc/passwd",
		"../../secret",
		"foo/../../..",
		"foo/../../../etc",
		"../",
		"..",
	}
	for _, rel := range traversals {
		t.Run(rel, func(t *testing.T) {
			_, err := ValidatePath(dataDir, rel)
			if err == nil {
				t.Errorf("expected error for traversal path %q, got nil", rel)
			}
		})
	}
}

func TestValidatePath_SymlinkEscape(t *testing.T) {
	dataDir := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(dataDir, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	_, err := ValidatePath(dataDir, "escape/secret.txt")
	if err == nil {
		t.Error("expected error when symlink escapes data directory, got nil")
	}
}

func TestValidatePath_SymlinkInsideDataDir(t *testing.T) {
	dataDir := t.TempDir()

	target := filepath.Join(dataDir, "real")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dataDir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	got, err := ValidatePath(dataDir, "link/file.md")
	if err != nil {
		t.Fatalf("symlink within dataDir should be allowed: %v", err)
	}
	expected := filepath.Join(target, "file.md")
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestValidateParentDir_AllowsRoot(t *testing.T) {
	dataDir := t.TempDir()

	got, err := ValidateParentDir(dataDir, "")
	if err != nil {
		t.Fatalf("empty relative path should resolve to dataDir: %v", err)
	}
	if got != dataDir {
		t.Errorf("got %q, want %q", got, dataDir)
	}
}

func TestValidateParentDir_AllowsChild(t *testing.T) {
	dataDir := t.TempDir()
	sub := filepath.Join(dataDir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ValidateParentDir(dataDir, "sub")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != sub {
		t.Errorf("got %q, want %q", got, sub)
	}
}

func TestValidateParentDir_RejectsTraversal(t *testing.T) {
	dataDir := t.TempDir()

	_, err := ValidateParentDir(dataDir, "../other")
	if err == nil {
		t.Error("expected error for traversal, got nil")
	}
}

func TestValidateParentDir_SymlinkEscape(t *testing.T) {
	dataDir := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(dataDir, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	_, err := ValidateParentDir(dataDir, "escape")
	if err == nil {
		t.Error("expected error when symlink escapes data directory, got nil")
	}
}

func TestValidatePath_DeepNonExistentPath(t *testing.T) {
	dataDir := t.TempDir()

	got, err := ValidatePath(dataDir, "a/b/c/d/e.md")
	if err != nil {
		t.Fatalf("deep non-existent path should be allowed: %v", err)
	}
	expected := filepath.Join(dataDir, "a/b/c/d/e.md")
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestMatchesFileType(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		fileType FileType
		want     bool
	}{
		// Valid note filenames
		{"valid note", "note_Meeting.md", NoteFileType, true},
		{"valid note with spaces", "note_My Notes.md", NoteFileType, true},
		{"valid note single char", "note_x.md", NoteFileType, true},
		{"valid note long title", "note_This is a very long title with many words.md", NoteFileType, true},

		// Valid task list filenames
		{"valid task list", "tasks_Sprint.md", TaskListFileType, true},
		{"valid task list single char", "tasks_Q.md", TaskListFileType, true},
		{"valid task list with spaces", "tasks_My Tasks.md", TaskListFileType, true},

		// Invalid - missing prefix
		{"missing note prefix", "Meeting.md", NoteFileType, false},
		{"missing task prefix", "Sprint.md", TaskListFileType, false},

		// Invalid - wrong prefix
		{"wrong prefix for note", "task_Meeting.md", NoteFileType, false},
		{"wrong prefix for task", "note_Sprint.md", TaskListFileType, false},

		// Invalid - missing suffix
		{"missing note suffix", "note_Meeting", NoteFileType, false},
		{"missing task suffix", "tasks_Sprint", TaskListFileType, false},

		// Invalid - wrong suffix
		{"wrong suffix for note", "note_Meeting.txt", NoteFileType, false},
		{"wrong suffix for task", "tasks_Sprint.txt", TaskListFileType, false},

		// Invalid - too short (no content between prefix and suffix)
		{"note too short", "note_.md", NoteFileType, false},
		{"task too short", "tasks_.md", TaskListFileType, false},

		// Invalid - empty string
		{"empty string note", "", NoteFileType, false},
		{"empty string task", "", TaskListFileType, false},

		// Invalid - only prefix
		{"only note prefix", "note_", NoteFileType, false},
		{"only task prefix", "tasks_", TaskListFileType, false},

		// Invalid - only suffix
		{"only suffix", ".md", NoteFileType, false},

		// Edge cases with special characters
		{"note with dots", "note_file.name.md", NoteFileType, true},
		{"note with underscores", "note_my_file_name.md", NoteFileType, true},
		{"task with dots", "tasks_file.name.md", TaskListFileType, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesFileType(tt.filename, tt.fileType)
			if got != tt.want {
				t.Errorf("MatchesFileType(%q, %v) = %v, want %v", tt.filename, tt.fileType.Label, got, tt.want)
			}
		})
	}
}
