package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
	"pgregory.net/rapid"
)

// Feature: task-management, Property 4: Created notes use note_ prefix
// For any valid title and content, after CreateNote, the file on disk must be named note_<title>.md.
// **Validates: Requirements 2.3, 2.4**
func TestProperty4_CreatedNotesUseNotePrefix(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()

		title := nameGen().Draw(rt, "title")
		content := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(rt, "content")

		srv := NewNotesServer(tmp)
		resp, err := srv.CreateNote(context.Background(), &pb.CreateNoteRequest{
			Title:   title,
			Content: content,
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		// Returned title should be the original (without prefix)
		if resp.Title != title {
			rt.Fatalf("expected title %q, got %q", title, resp.Title)
		}

		// File on disk must be note_<title>.md
		expectedFile := "note_" + title + ".md"
		absPath := filepath.Join(tmp, expectedFile)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			rt.Fatalf("expected file %q to exist on disk", expectedFile)
		}

		// FilePath in response should reflect the prefix
		if resp.FilePath != expectedFile {
			rt.Fatalf("expected file_path %q, got %q", expectedFile, resp.FilePath)
		}
	})
}
