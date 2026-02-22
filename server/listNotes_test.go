package server

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
)

func TestListNotes_ShallowListing(t *testing.T) {
	tmp := t.TempDir()

	// Create a .md file at root
	if err := os.WriteFile(filepath.Join(tmp, "note1.md"), []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a subdirectory with a nested note (should NOT appear in root listing)
	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "note2.md"), []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}
	// Create a non-.md file (should appear in entries? No — only folders and .md files matter)
	if err := os.WriteFile(filepath.Join(tmp, "ignore.txt"), []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewNotesServer(tmp)

	resp, err := s.ListNotes(context.Background(), &pb.ListNotesRequest{})
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}

	// Shallow: only note1.md as a Note (not sub/note2.md)
	if len(resp.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(resp.Notes))
	}
	if resp.Notes[0].FilePath != "note1.md" {
		t.Fatalf("unexpected FilePath: %s", resp.Notes[0].FilePath)
	}
	if resp.Notes[0].Title != "note1" {
		t.Fatalf("unexpected title: %s", resp.Notes[0].Title)
	}
	if resp.Notes[0].Content != "content1" {
		t.Fatalf("unexpected content: %s", resp.Notes[0].Content)
	}

	// Entries should contain folder "sub/" and note "note1.md" (not ignore.txt)
	sort.Strings(resp.Entries)
	expected := []string{"note1.md", "sub/"}
	if len(resp.Entries) != len(expected) {
		t.Fatalf("expected entries %v, got %v", expected, resp.Entries)
	}
	for i, e := range expected {
		if resp.Entries[i] != e {
			t.Fatalf("entry[%d]: expected %q, got %q", i, e, resp.Entries[i])
		}
	}
}

func TestListNotes_SubfolderPath(t *testing.T) {
	tmp := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmp, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "sub", "note2.md"), []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}

	s := NewNotesServer(tmp)

	resp, err := s.ListNotes(context.Background(), &pb.ListNotesRequest{Path: "sub"})
	if err != nil {
		t.Fatalf("ListNotes with Path failed: %v", err)
	}
	if len(resp.Notes) != 1 {
		t.Fatalf("expected 1 note for sub, got %d", len(resp.Notes))
	}
	if resp.Notes[0].FilePath != "sub/note2.md" {
		t.Fatalf("unexpected FilePath: %s", resp.Notes[0].FilePath)
	}

	// Entries should include the note with sub/ prefix
	if len(resp.Entries) != 1 || resp.Entries[0] != "sub/note2.md" {
		t.Fatalf("expected entries [sub/note2.md], got %v", resp.Entries)
	}
}

func TestListNotes_FolderEntriesTrailingSlash(t *testing.T) {
	tmp := t.TempDir()

	if err := os.MkdirAll(filepath.Join(tmp, "FolderA"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "FolderB"), 0755); err != nil {
		t.Fatal(err)
	}

	s := NewNotesServer(tmp)

	resp, err := s.ListNotes(context.Background(), &pb.ListNotesRequest{})
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}

	if len(resp.Notes) != 0 {
		t.Fatalf("expected 0 notes, got %d", len(resp.Notes))
	}

	sort.Strings(resp.Entries)
	expected := []string{"FolderA/", "FolderB/"}
	if len(resp.Entries) != len(expected) {
		t.Fatalf("expected entries %v, got %v", expected, resp.Entries)
	}
	for i, e := range expected {
		if resp.Entries[i] != e {
			t.Fatalf("entry[%d]: expected %q, got %q", i, e, resp.Entries[i])
		}
	}
}
