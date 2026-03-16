package notes

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
)

func TestListNotes_ShallowListing(t *testing.T) {
	tmp := t.TempDir()

	// Create a note_ prefixed .md file at root
	if err := os.WriteFile(filepath.Join(tmp, "note_note1.md"), []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a subdirectory with a nested note (should NOT appear in root listing)
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "note_note2.md"), []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a non-.md file (should not appear in notes)
	if err := os.WriteFile(filepath.Join(tmp, "ignore.txt"), []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewNotesServer(tmp, nopLogger())

	resp, err := s.ListNotes(context.Background(), &pb.ListNotesRequest{})
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}

	// Shallow: only note_note1.md as a Note (not sub/note_note2.md)
	if len(resp.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(resp.Notes))
	}
	if resp.Notes[0].FilePath != "note_note1.md" {
		t.Fatalf("unexpected FilePath: %s", resp.Notes[0].FilePath)
	}
	if resp.Notes[0].Title != "note1" {
		t.Fatalf("unexpected title: %s", resp.Notes[0].Title)
	}
	if resp.Notes[0].Content != "content1" {
		t.Fatalf("unexpected content: %s", resp.Notes[0].Content)
	}
}

func TestListNotes_SubfolderPath(t *testing.T) {
	tmp := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "note_note2.md"), []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewNotesServer(tmp, nopLogger())

	resp, err := s.ListNotes(context.Background(), &pb.ListNotesRequest{ParentDir: "sub"})
	if err != nil {
		t.Fatalf("ListNotes with Path failed: %v", err)
	}
	if len(resp.Notes) != 1 {
		t.Fatalf("expected 1 note for sub, got %d", len(resp.Notes))
	}
	if resp.Notes[0].FilePath != "sub/note_note2.md" {
		t.Fatalf("unexpected FilePath: %s", resp.Notes[0].FilePath)
	}
}


func TestListNotes_OrphanNoteReturnsEmptyId(t *testing.T) {
	tmp := t.TempDir()

	// Write a note file directly on disk — no registry entry.
	if err := os.WriteFile(filepath.Join(tmp, "note_Orphan.md"), []byte("orphan content"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewNotesServer(tmp, nopLogger())

	resp, err := s.ListNotes(context.Background(), &pb.ListNotesRequest{})
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}

	if len(resp.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(resp.Notes))
	}

	note := resp.Notes[0]
	if note.Id != "" {
		t.Fatalf("expected empty id for orphan note, got %q", note.Id)
	}
	if note.Title != "Orphan" {
		t.Fatalf("expected title 'Orphan', got %q", note.Title)
	}
	if note.Content != "orphan content" {
		t.Fatalf("expected content 'orphan content', got %q", note.Content)
	}
}

