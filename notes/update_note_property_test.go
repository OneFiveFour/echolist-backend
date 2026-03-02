package notes

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

// Feature: code-review-hardening, Property 4: UpdateNote rejects non-existent files
// For any file path that does not exist on disk, calling UpdateNote should return
// a Connect error with code CodeNotFound and should not create any new file.
// **Validates: Requirements 3.1, 3.2**
func TestProperty_UpdateNoteRejectsNonExistent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewNotesServer(tmp)
		ctx := context.Background()

		// Generate a valid-looking note file path that doesn't exist on disk.
		// Optionally nest under a subdirectory to cover both flat and nested cases.
		title := nameGen().Draw(rt, "title")
		useSubdir := rapid.Bool().Draw(rt, "useSubdir")

		var filePath string
		if useSubdir {
			subdir := nameGen().Draw(rt, "subdir")
			filePath = filepath.Join(subdir, "note_"+title+".md")
		} else {
			filePath = "note_" + title + ".md"
		}

		_, err := srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
			FilePath: filePath,
			Content:  "some content",
		})

		// Should return CodeNotFound
		if err == nil {
			rt.Fatalf("UpdateNote: expected error for non-existent path %q, got nil", filePath)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("UpdateNote: expected connect.Error, got %T: %v", err, err)
		}
		if connErr.Code() != connect.CodeNotFound {
			rt.Fatalf("UpdateNote: expected CodeNotFound for path %q, got %v", filePath, connErr.Code())
		}

		// Verify no file was created at that path
		absPath := filepath.Join(tmp, filePath)
		if _, statErr := os.Stat(absPath); !os.IsNotExist(statErr) {
			rt.Fatalf("UpdateNote: file should not exist at %q", absPath)
		}
	})
}
