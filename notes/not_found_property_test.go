package notes

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"

	pb "echolist-backend/proto/gen/notes/v1"

	"pgregory.net/rapid"
)

// uuidV4Gen generates valid UUIDv4 strings that were never used in CreateNote.
func uuidV4Gen() *rapid.Generator[string] {
	return rapid.Custom(func(rt *rapid.T) string {
		// Generate random hex segments with correct version (4) and variant ([89ab]) bits
		a := rapid.StringMatching(`[0-9a-f]{8}`).Draw(rt, "a")
		b := rapid.StringMatching(`[0-9a-f]{4}`).Draw(rt, "b")
		c := rapid.StringMatching(`[0-9a-f]{3}`).Draw(rt, "c")
		d := rapid.StringMatching(`[89ab][0-9a-f]{3}`).Draw(rt, "d")
		e := rapid.StringMatching(`[0-9a-f]{12}`).Draw(rt, "e")
		return fmt.Sprintf("%s-%s-4%s-%s-%s", a, b, c, d, e)
	})
}

// Feature: note-stable-ids, Property 6: Non-existent id returns NotFound
// For any valid UUIDv4 string that was never used in a CreateNote call,
// calling GetNote, UpdateNote, or DeleteNote with that id shall return a NotFound error.
// **Validates: Requirements 4.2, 5.2, 6.2**
func TestProperty_NotFoundReturnsCodeNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := uuidV4Gen().Draw(rt, "id")
		tmpDir := t.TempDir()
		srv := NewNotesServer(tmpDir, testDB(t), nopLogger())
		ctx := context.Background()

		// GetNote with non-existent id should return CodeNotFound
		_, err := srv.GetNote(ctx, &pb.GetNoteRequest{
			Id: id,
		})
		assertCodeNotFound(rt, err, "GetNote", id)

		// UpdateNote with non-existent id should return CodeNotFound
		_, err = srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
			Id:      id,
			Title:   "some title",
			Content: "some content",
		})
		assertCodeNotFound(rt, err, "UpdateNote", id)

		// DeleteNote with non-existent id should return CodeNotFound
		_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{
			Id: id,
		})
		assertCodeNotFound(rt, err, "DeleteNote", id)
	})
}

// assertCodeNotFound checks that the error is a connect.Error with CodeNotFound.
func assertCodeNotFound(rt *rapid.T, err error, handler, id string) {
	rt.Helper()
	if err == nil {
		rt.Fatalf("%s: expected error for non-existent id %q, got nil", handler, id)
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		rt.Fatalf("%s: expected connect.Error for id %q, got %T: %v", handler, id, err, err)
	}
	if connErr.Code() != connect.CodeNotFound {
		rt.Fatalf("%s: expected CodeNotFound for id %q, got %v", handler, id, connErr.Code())
	}
}


// Feature: note-stable-ids, Property 8: Invalid UUID returns InvalidArgument (RPC level)
// For any string that is not a valid UUIDv4, calling GetNote, UpdateNote, or DeleteNote
// with that string as the id shall return an InvalidArgument error.
// **Validates: Requirements 9.1, 9.2**
func TestProperty8_InvalidUuidRejectedByRpcs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := invalidUuidGen().Draw(rt, "invalidUuid")
		tmpDir := t.TempDir()
		srv := NewNotesServer(tmpDir, testDB(t), nopLogger())
		ctx := context.Background()

		// GetNote with invalid UUID should return CodeInvalidArgument
		_, err := srv.GetNote(ctx, &pb.GetNoteRequest{
			Id: id,
		})
		assertCodeInvalidArgument(rt, err, "GetNote", id)

		// UpdateNote with invalid UUID should return CodeInvalidArgument
		_, err = srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
			Id:      id,
			Title:   "some title",
			Content: "some content",
		})
		assertCodeInvalidArgument(rt, err, "UpdateNote", id)

		// DeleteNote with invalid UUID should return CodeInvalidArgument
		_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{
			Id: id,
		})
		assertCodeInvalidArgument(rt, err, "DeleteNote", id)
	})
}

