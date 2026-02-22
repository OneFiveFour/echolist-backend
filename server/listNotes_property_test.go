package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
	"pgregory.net/rapid"
)

// nameGen generates valid filesystem entry names: alphanumeric with hyphens/underscores, 1-30 chars.
func nameGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]{0,29}`)
}

// Property 7: ListNotes returns all immediate children with correct formatting
// For any set of folders and .md files created in a directory, ListNotes returns
// entries where folders have trailing "/" and notes don't, and every immediate
// child is represented.
// **Validates: Requirements 4.1, 4.2**
func TestProperty7_ListNotesImmediateChildrenFormatting(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()

		// Generate a random set of folder names and note names
		numFolders := rapid.IntRange(0, 5).Draw(rt, "numFolders")
		numNotes := rapid.IntRange(0, 5).Draw(rt, "numNotes")

		folderNames := make(map[string]bool)
		for i := 0; i < numFolders; i++ {
			name := nameGen().Draw(rt, "folderName")
			if folderNames[name] || strings.HasSuffix(name, ".md") {
				continue
			}
			folderNames[name] = true
			if err := os.MkdirAll(filepath.Join(tmp, name), 0755); err != nil {
				rt.Fatal(err)
			}
		}

		noteNames := make(map[string]bool)
		for i := 0; i < numNotes; i++ {
			name := nameGen().Draw(rt, "noteName")
			if noteNames[name] || folderNames[name] {
				continue
			}
			noteNames[name] = true
			if err := os.WriteFile(filepath.Join(tmp, name+".md"), []byte("content"), 0644); err != nil {
				rt.Fatal(err)
			}
		}

		srv := NewNotesServer(tmp)
		resp, err := srv.ListNotes(context.Background(), &pb.ListNotesRequest{})
		if err != nil {
			rt.Fatalf("ListNotes failed: %v", err)
		}

		// Build a set of returned entries
		entrySet := make(map[string]bool)
		for _, e := range resp.Entries {
			entrySet[e] = true
		}

		// Every folder must appear with trailing "/"
		for name := range folderNames {
			if !entrySet[name+"/"] {
				rt.Fatalf("folder %q missing from entries (expected %q)", name, name+"/")
			}
		}

		// Every note must appear without trailing "/"
		for name := range noteNames {
			if !entrySet[name+".md"] {
				rt.Fatalf("note %q missing from entries (expected %q)", name, name+".md")
			}
		}

		// Entries count should match folders + notes (no extra entries)
		expectedCount := len(folderNames) + len(noteNames)
		if len(resp.Entries) != expectedCount {
			rt.Fatalf("expected %d entries, got %d: %v", expectedCount, len(resp.Entries), resp.Entries)
		}

		// Notes slice should match note count
		if len(resp.Notes) != len(noteNames) {
			rt.Fatalf("expected %d notes, got %d", len(noteNames), len(resp.Notes))
		}

		// Folder entries must end with "/", note entries must not
		for _, e := range resp.Entries {
			if folderNames[strings.TrimSuffix(e, "/")] {
				if !strings.HasSuffix(e, "/") {
					rt.Fatalf("folder entry %q should end with /", e)
				}
			}
		}
	})
}

// Property 8: ListNotes shallow listing
// Files in subdirectories must NOT appear in the parent listing.
// Only immediate children are returned.
// **Validates: Requirements 4.3**
func TestProperty8_ListNotesShallowListing(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()

		folderName := nameGen().Draw(rt, "folderName")
		nestedNoteName := nameGen().Draw(rt, "nestedNoteName")
		rootNoteName := nameGen().Draw(rt, "rootNoteName")

		// Ensure folder name and nested note name don't collide
		if folderName == nestedNoteName {
			return
		}

		// Ensure names don't collide
		if folderName == rootNoteName {
			return
		}

		// Create a subfolder with a nested note
		subDir := filepath.Join(tmp, folderName)
		if err := os.MkdirAll(subDir, 0755); err != nil {
			rt.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, nestedNoteName+".md"), []byte("nested"), 0644); err != nil {
			rt.Fatal(err)
		}

		// Create a root-level note
		if err := os.WriteFile(filepath.Join(tmp, rootNoteName+".md"), []byte("root"), 0644); err != nil {
			rt.Fatal(err)
		}

		srv := NewNotesServer(tmp)
		resp, err := srv.ListNotes(context.Background(), &pb.ListNotesRequest{})
		if err != nil {
			rt.Fatalf("ListNotes failed: %v", err)
		}

		// The nested note must NOT appear in root listing notes
		for _, n := range resp.Notes {
			if strings.Contains(n.FilePath, folderName+"/") {
				rt.Fatalf("nested note %q should not appear in shallow root listing", n.FilePath)
			}
		}

		// The nested note path (folder/name.md) must NOT appear in root entries
		nestedPath := folderName + "/" + nestedNoteName + ".md"
		for _, e := range resp.Entries {
			if e == nestedPath {
				rt.Fatalf("nested note path %q should not appear in root entries: %v", nestedPath, resp.Entries)
			}
		}

		// Root listing should have exactly 2 entries: the folder and the root note
		if len(resp.Entries) != 2 {
			rt.Fatalf("expected 2 entries, got %d: %v", len(resp.Entries), resp.Entries)
		}
	})
}
