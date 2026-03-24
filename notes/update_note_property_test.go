package notes

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	pb "echolist-backend/proto/gen/notes/v1"
	"pgregory.net/rapid"
)

// Feature: code-review-hardening, Property 4: UpdateNote rejects non-existent files
// For any file path that does not exist on disk, calling UpdateNote should return
// a Connect error with code CodeNotFound and should not create any new file.
// **Validates: Requirements 3.1, 3.2**
// Feature: note-stable-ids, Property 6 (partial): UpdateNote rejects non-existent ids
// For any valid UUIDv4 that was never used in a CreateNote call, calling UpdateNote
// should return a Connect error with code CodeNotFound.
// **Validates: Requirements 5.2**
func TestProperty_UpdateNoteRejectsNonExistent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewNotesServer(tmp, nopLogger())
		ctx := context.Background()

		// Generate a valid UUIDv4 that was never used in CreateNote
		id := uuidV4Gen().Draw(rt, "id")

		_, err := srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
			Id:      id,
			Content: "some content",
		})

		// Should return CodeNotFound
		if err == nil {
			rt.Fatalf("UpdateNote: expected error for non-existent id %q, got nil", id)
		}
		var connErr *connect.Error
		if !errors.As(err, &connErr) {
			rt.Fatalf("UpdateNote: expected connect.Error, got %T: %v", err, err)
		}
		if connErr.Code() != connect.CodeNotFound {
			rt.Fatalf("UpdateNote: expected CodeNotFound for id %q, got %v", id, connErr.Code())
		}
	})
}

// Feature: note-stable-ids, Property 4: Update by id preserves the Note_Id
// For any created note and any new content string, calling UpdateNote with the
// note's id shall return a Note whose id is identical to the original, and whose
// content matches the new content.
// **Validates: Requirements 1.4, 5.1**
func TestProperty4_UpdateByIdPreservesNoteId(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewNotesServer(tmp, nopLogger())
		ctx := context.Background()

		title := nameGen().Draw(rt, "title")
		originalContent := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(rt, "originalContent")
		newContent := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(rt, "newContent")

		// Create a note to get a valid id
		createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			Title:   title,
			Content: originalContent,
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		originalId := createResp.Note.Id

		// Update the note with new content using the same id
		updateResp, err := srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
			Id:      originalId,
			Content: newContent,
		})
		if err != nil {
			rt.Fatalf("UpdateNote failed: %v", err)
		}

		// The returned note's id must be identical to the original
		if updateResp.Note.Id != originalId {
			rt.Fatalf("expected id %q after update, got %q", originalId, updateResp.Note.Id)
		}

		// The returned note's content must match the new content
		if updateResp.Note.Content != newContent {
			rt.Fatalf("expected content %q after update, got %q", newContent, updateResp.Note.Content)
		}
	})
}
