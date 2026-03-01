package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

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
		if resp.Note.Title != title {
			rt.Fatalf("expected title %q, got %q", title, resp.Note.Title)
		}

		// File on disk must be note_<title>.md
		expectedFile := "note_" + title + ".md"
		absPath := filepath.Join(tmp, expectedFile)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			rt.Fatalf("expected file %q to exist on disk", expectedFile)
		}

		// FilePath in response should reflect the prefix
		if resp.Note.FilePath != expectedFile {
			rt.Fatalf("expected file_path %q, got %q", expectedFile, resp.Note.FilePath)
		}
	})
}

// titleWithPathSepGen generates a random string that contains at least one `/` or `\`.
// Strategy: generate a base string, pick a random separator, and insert it at a random position.
func titleWithPathSepGen() *rapid.Generator[string] {
	return rapid.Custom(func(rt *rapid.T) string {
		base := rapid.StringMatching(`[a-zA-Z0-9]{0,20}`).Draw(rt, "base")
		sep := rapid.SampledFrom([]string{"/", "\\"}).Draw(rt, "sep")
		pos := rapid.IntRange(0, len(base)).Draw(rt, "pos")
		return base[:pos] + sep + base[pos:]
	})
}

// Feature: api-hardening-cleanup, Property 5: Titles containing path separators are rejected
// For any title string containing `/` or `\` characters, calling CreateNote
// should return a Connect error with code CodeInvalidArgument.
// **Validates: Requirements 5.2**
func TestProperty_TitleWithPathSeparatorsRejected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		title := titleWithPathSepGen().Draw(rt, "title")
		tmpDir := t.TempDir()
		srv := NewNotesServer(tmpDir)
		ctx := context.Background()

		_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			Path:    "",
			Title:   title,
			Content: "test content",
		})

		if err == nil {
			rt.Fatalf("CreateNote: expected error for title %q containing path separator, got nil", title)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("CreateNote: expected connect.Error for title %q, got %T: %v", title, err, err)
		}
		if connErr.Code() != connect.CodeInvalidArgument {
			rt.Fatalf("CreateNote: expected CodeInvalidArgument for title %q, got %v", title, connErr.Code())
		}
	})
}

