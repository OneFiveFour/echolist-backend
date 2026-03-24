package notes

import (
	"context"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
)

func TestGetNote(t *testing.T) {
	tmp := t.TempDir()
	s := NewNotesServer(tmp, nopLogger())

	// Create a note via the RPC to get a valid id
	createResp, err := s.CreateNote(context.Background(), &pb.CreateNoteRequest{
		Title:   "mytest",
		Content: "abc",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	resp, err := s.GetNote(context.Background(), &pb.GetNoteRequest{Id: createResp.Note.Id})
	if err != nil {
		t.Fatalf("GetNote failed: %v", err)
	}
	if resp.Note.FilePath != "note_mytest.md" {
		t.Fatalf("unexpected FilePath: %s", resp.Note.FilePath)
	}
	if resp.Note.Title != "mytest" {
		t.Fatalf("unexpected Title: %s", resp.Note.Title)
	}
	if resp.Note.Content != "abc" {
		t.Fatalf("unexpected Content: %s", resp.Note.Content)
	}
	if resp.Note.UpdatedAt <= 0 {
		t.Fatalf("invalid UpdatedAt: %d", resp.Note.UpdatedAt)
	}
	if resp.Note.Id != createResp.Note.Id {
		t.Fatalf("unexpected Id: got %s, want %s", resp.Note.Id, createResp.Note.Id)
	}
}
