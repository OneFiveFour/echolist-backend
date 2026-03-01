package server

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"

	pb "echolist-backend/proto/gen/notes/v1"

	"pgregory.net/rapid"
)

// nonExistentFilePathGen generates valid note file paths (no path traversal)
// that will not exist on disk in an empty temp directory.
func nonExistentFilePathGen() *rapid.Generator[string] {
	return rapid.Custom(func(rt *rapid.T) string {
		name := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]{0,19}`).Draw(rt, "name")
		return "note_" + name + ".md"
	})
}

// Feature: api-hardening-cleanup, Property 4: Non-existent file returns CodeNotFound
// For any file path that does not exist on disk, calling GetNote or DeleteNote
// should return a Connect error with code CodeNotFound.
// **Validates: Requirements 4.1, 4.2**
func TestProperty_NotFoundReturnsCodeNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		filePath := nonExistentFilePathGen().Draw(rt, "filePath")
		tmpDir := t.TempDir()
		srv := NewNotesServer(tmpDir)
		ctx := context.Background()

		// GetNote with non-existent file should return CodeNotFound
		_, err := srv.GetNote(ctx, &pb.GetNoteRequest{
			FilePath: filePath,
		})
		assertCodeNotFound(rt, err, "GetNote", filePath)

		// DeleteNote with non-existent file should return CodeNotFound
		_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{
			FilePath: filePath,
		})
		assertCodeNotFound(rt, err, "DeleteNote", filePath)
	})
}

// assertCodeNotFound checks that the error is a connect.Error with CodeNotFound.
func assertCodeNotFound(rt *rapid.T, err error, handler, path string) {
	rt.Helper()
	if err == nil {
		rt.Fatalf("%s: expected error for non-existent path %q, got nil", handler, path)
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		rt.Fatalf("%s: expected connect.Error for path %q, got %T: %v", handler, path, err, err)
	}
	if connErr.Code() != connect.CodeNotFound {
		rt.Fatalf("%s: expected CodeNotFound for path %q, got %v", handler, path, connErr.Code())
	}
}
