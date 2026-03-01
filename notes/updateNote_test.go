package notes

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
)

func TestUpdateNote(t *testing.T) {
	tmp := t.TempDir()
	s := NewNotesServer(tmp)

	// Ensure target directory and file exist
	if err := os.MkdirAll(filepath.Join(tmp, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	notePath := filepath.Join(tmp, "a", "note_b.md")
	if err := os.WriteFile(notePath, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	req := &pb.UpdateNoteRequest{FilePath: filepath.Join("a", "note_b.md"), Content: "hello"}
	resp, err := s.UpdateNote(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateNote failed: %v", err)
	}
	if resp.Note.UpdatedAt <= 0 {
		t.Fatalf("invalid UpdatedAt: %d", resp.Note.UpdatedAt)
	}

	// verify file contents
	b, err := os.ReadFile(filepath.Join(tmp, req.FilePath))
	if err != nil {
		t.Fatalf("reading written file failed: %v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("unexpected file content: %s", string(b))
	}
}
