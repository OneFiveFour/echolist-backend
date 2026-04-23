package notes

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	pb "echolist-backend/proto/gen/notes/v1"
	"pgregory.net/rapid"
)

// Feature: note-stable-ids, Property 5: Delete by id removes file and registry entry
// For any created note, calling DeleteNote with its id shall succeed, and a
// subsequent GetNote with the same id shall return NotFound.
// **Validates: Requirements 2.2, 6.1**
func TestProperty5_DeleteByIdRemovesFileAndRegistryEntry(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewNotesServer(tmp, testDB(t), nopLogger())
		ctx := context.Background()

		title := nameGen().Draw(rt, "title")
		content := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(rt, "content")

		// Create a note to get a valid id
		createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			Title:   title,
			Content: content,
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		noteId := createResp.Note.Id

		// Delete the note by id — should succeed
		_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{
			Id: noteId,
		})
		if err != nil {
			rt.Fatalf("DeleteNote failed for id %q: %v", noteId, err)
		}

		// GetNote with the same id should return NotFound
		_, err = srv.GetNote(ctx, &pb.GetNoteRequest{
			Id: noteId,
		})
		if err == nil {
			rt.Fatalf("GetNote: expected NotFound error for deleted id %q, got nil", noteId)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("GetNote: expected connect.Error, got %T: %v", err, err)
		}
		if connErr.Code() != connect.CodeNotFound {
			rt.Fatalf("GetNote: expected CodeNotFound for deleted id %q, got %v", noteId, connErr.Code())
		}
	})
}
