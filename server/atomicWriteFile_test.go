package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicWriteFile_Success(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(targetDir, "note.md")
	data := []byte("hello-atomic")

	if err := atomicWriteFile(path, data); err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file failed: %v", err)
	}
	if string(b) != string(data) {
		t.Fatalf("unexpected content: %s", string(b))
	}

	// ensure no stray temp files remain
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		t.Fatalf("read dir failed: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Fatalf("found leftover temp file: %s", e.Name())
		}
	}
}

func TestAtomicWriteFile_NoDir(t *testing.T) {
	tmp := t.TempDir()
	// do not create the directory
	path := filepath.Join(tmp, "doesnotexist", "note.md")
	if err := atomicWriteFile(path, []byte("x")); err == nil {
		t.Fatalf("expected error when directory does not exist")
	}
}
