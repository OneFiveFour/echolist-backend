package notes

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "echolist-backend/proto/gen/notes/v1"

	"pgregory.net/rapid"
)

// traversalPathGen generates path strings containing ".." segments that escape the data directory.
func traversalPathGen() *rapid.Generator[string] {
	return rapid.Custom(func(rt *rapid.T) string {
		numSegments := rapid.IntRange(1, 5).Draw(rt, "numSegments")
		segments := make([]string, numSegments)
		for i := range segments {
			segments[i] = ".."
		}
		base := strings.Join(segments, "/")

		suffix := rapid.SampledFrom([]string{"", "/etc/passwd", "/tmp", "/var/log"}).Draw(rt, "suffix")
		return base + suffix
	})
}

// Feature: api-hardening-cleanup, Property 3: Path traversal rejection across NoteService
// For any path that resolves outside the data directory (containing ".." segments),
// all NoteService handlers (CreateNote, GetNote, UpdateNote, DeleteNote, ListNotes)
// should return a Connect error with code CodeInvalidArgument.
// **Validates: Requirements 3.1, 3.2, 3.3, 3.4**
func TestProperty_PathTraversalRejection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		traversalPath := traversalPathGen().Draw(rt, "traversalPath")
		tmpDir := t.TempDir()
		srv := NewNotesServer(tmpDir, nopLogger())
		ctx := context.Background()

		// CreateNote: path field is the directory
		_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			ParentDir: traversalPath,
			Title:   "test",
			Content: "test",
		})
		assertCodeInvalidArgument(rt, err, "CreateNote", traversalPath)

		// GetNote: file_path field
		_, err = srv.GetNote(ctx, &pb.GetNoteRequest{
			Id: traversalPath,
		})
		assertCodeInvalidArgument(rt, err, "GetNote", traversalPath)

		// UpdateNote: file_path field
		_, err = srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
			Id: traversalPath,
			Content:  "test",
		})
		assertCodeInvalidArgument(rt, err, "UpdateNote", traversalPath)

		// DeleteNote: file_path field
		_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{
			Id: traversalPath,
		})
		assertCodeInvalidArgument(rt, err, "DeleteNote", traversalPath)

		// ListNotes: path field is the directory
		_, err = srv.ListNotes(ctx, &pb.ListNotesRequest{
			ParentDir: traversalPath,
		})
		assertCodeInvalidArgument(rt, err, "ListNotes", traversalPath)
	})
}

// assertCodeInvalidArgument checks that the error is a connect.Error with CodeInvalidArgument.
func assertCodeInvalidArgument(rt *rapid.T, err error, handler, path string) {
	rt.Helper()
	if err == nil {
		rt.Fatalf("%s: expected error for traversal path %q, got nil", handler, path)
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		rt.Fatalf("%s: expected connect.Error for path %q, got %T: %v", handler, path, err, err)
	}
	if connErr.Code() != connect.CodeInvalidArgument {
		rt.Fatalf("%s: expected CodeInvalidArgument for path %q, got %v", handler, path, connErr.Code())
	}
}
