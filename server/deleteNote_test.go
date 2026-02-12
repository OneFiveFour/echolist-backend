package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "notes-backend/proto/gen/notes/v1"
)

func TestDeleteNote(t *testing.T) {
	tmp := t.TempDir()
	s := NewNotesServer(tmp)

	path := filepath.Join(tmp, "todelete.md")
	if err := os.WriteFile(path, []byte("bye"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := s.DeleteNote(context.Background(), &pb.DeleteNoteRequest{FilePath: "todelete.md"})
	if err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	if _, err := os.Stat(path); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, stat error: %v", err)
	}
}
