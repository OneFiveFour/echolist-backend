package pathutil

import (
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
)

func TestRequireDir_ExistingDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := RequireDir(dir, "folder"); err != nil {
		t.Fatalf("expected no error for existing directory, got %v", err)
	}
}

func TestRequireDir_NonExistentPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope")
	err := RequireDir(path, "folder")
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestRequireDir_FileNotDirectory(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "afile.txt")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := RequireDir(p, "parent directory")
	if err == nil {
		t.Fatal("expected error when path is a file, not a directory")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}
