package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "notes-backend/gen/notes"
)

func TestListNotes(t *testing.T) {
	tmp := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmp, "note1.md"), []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "note2.md"), []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "ignore.txt"), []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewNotesServer(tmp)

	resp, err := s.ListNotes(context.Background(), &pb.ListNotesRequest{})
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}
	if len(resp.Notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(resp.Notes))
	}

	m := make(map[string]*pb.Note)
	for _, n := range resp.Notes {
		m[n.FilePath] = n
	}

	n1, ok := m[filepath.Join("note1.md")]
	if !ok {
		t.Fatalf("note1.md not found")
	}
	if n1.Title != "note1" {
		t.Fatalf("unexpected title for note1: %s", n1.Title)
	}
	if n1.Content != "content1" {
		t.Fatalf("unexpected content for note1: %s", n1.Content)
	}
	if n1.UpdatedAt <= 0 {
		t.Fatalf("invalid UpdatedAt for note1: %d", n1.UpdatedAt)
	}

	n2, ok := m[filepath.Join("sub", "note2.md")]
	if !ok {
		t.Fatalf("sub/note2.md not found")
	}
	if n2.Title != "note2" {
		t.Fatalf("unexpected title for note2: %s", n2.Title)
	}
	if n2.Content != "content2" {
		t.Fatalf("unexpected content for note2: %s", n2.Content)
	}

	// Verify filtering by Path
	resp2, err := s.ListNotes(context.Background(), &pb.ListNotesRequest{Path: "sub"})
	if err != nil {
		t.Fatalf("ListNotes with Path failed: %v", err)
	}
	if len(resp2.Notes) != 1 {
		t.Fatalf("expected 1 note for sub, got %d", len(resp2.Notes))
	}
	if resp2.Notes[0].FilePath != filepath.Join("sub", "note2.md") {
		t.Fatalf("unexpected FilePath: %s", resp2.Notes[0].FilePath)
	}
}
