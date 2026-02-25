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
	if resp.FilePath != "note_mytest.md" {
		t.Fatalf("unexpected FilePath: %s", resp.FilePath)
	}
	if resp.Title != "mytest" {
		t.Fatalf("unexpected Title: %s", resp.Title)
	}
	if resp.Content != "abc" {
		t.Fatalf("unexpected Content: %s", resp.Content)
	}
	if resp.UpdatedAt <= 0 {
		t.Fatalf("invalid UpdatedAt: %d", resp.UpdatedAt)
	}
}
