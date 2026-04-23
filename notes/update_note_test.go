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
	s := NewNotesServer(tmp, testDB(t), nopLogger())

	// Create a note via the RPC to get a valid id
	createResp, err := s.CreateNote(context.Background(), &pb.CreateNoteRequest{
		Title:   "b",
		Content: "old content",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	noteId := createResp.Note.Id

	req := &pb.UpdateNoteRequest{Id: noteId, Title: "renamed", Content: "hello"}
	resp, err := s.UpdateNote(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateNote failed: %v", err)
	}
	if resp.Note.UpdatedAt <= 0 {
		t.Fatalf("invalid UpdatedAt: %d", resp.Note.UpdatedAt)
	}
	if resp.Note.Id != noteId {
		t.Fatalf("unexpected Id: got %s, want %s", resp.Note.Id, noteId)
	}
	if resp.Note.Title != "renamed" {
		t.Fatalf("unexpected Title: got %s, want %s", resp.Note.Title, "renamed")
	}

	// verify file contents: old file should be gone
	oldFile := filepath.Join(tmp, "note_b.md")
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old file to be gone, stat err=%v", err)
	}
	newFile := filepath.Join(tmp, "note_renamed.md")
	b, err := os.ReadFile(newFile)
	if err != nil {
		t.Fatalf("reading written file failed: %v", err)
	}
	if string(b) != "hello" {
		t.Fatalf("unexpected file content: %s", string(b))
	}

	got, err := s.GetNote(context.Background(), &pb.GetNoteRequest{Id: noteId})
	if err != nil {
		t.Fatalf("GetNote after update failed: %v", err)
	}
	if got.Note.Title != "renamed" {
		t.Fatalf("unexpected title after get: got %s, want %s", got.Note.Title, "renamed")
	}
}
