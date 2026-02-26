package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
)

func TestGetNote(t *testing.T) {
	tmp := t.TempDir()
	s := NewNotesServer(tmp)

	if err := os.WriteFile(filepath.Join(tmp, "note_mytest.md"), []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}

	resp, err := s.GetNote(context.Background(), &pb.GetNoteRequest{FilePath: "note_mytest.md"})
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
}
