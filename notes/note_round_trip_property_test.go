package notes

import (
	"context"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
	"pgregory.net/rapid"
)

// Feature: api-unification, Property 1: Note create-then-get round trip
// For any valid note title and content, creating a note via CreateNote and then
// retrieving it via GetNote using the returned file_path should produce a Note
// with the same title, content, and file_path. The updated_at should be non-zero
// in both responses.
// **Validates: Requirements 3.1, 3.2, 3.4, 3.5, 4.1**
func TestProperty1_NoteCreateThenGetRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewNotesServer(tmp, nopLogger())

		title := nameGen().Draw(rt, "title")
		content := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(rt, "content")

		createResp, err := srv.CreateNote(context.Background(), &pb.CreateNoteRequest{
			Title:   title,
			Content: content,
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		created := createResp.Note
		if created == nil {
			rt.Fatal("CreateNote returned nil Note")
		}
		if created.Title != title {
			rt.Fatalf("create: expected title %q, got %q", title, created.Title)
		}
		if created.Content != content {
			rt.Fatalf("create: expected content %q, got %q", content, created.Content)
		}
		if created.UpdatedAt == 0 {
			rt.Fatal("create: updated_at should be non-zero")
		}

		getResp, err := srv.GetNote(context.Background(), &pb.GetNoteRequest{
			Id: created.FilePath,
		})
		if err != nil {
			rt.Fatalf("GetNote failed: %v", err)
		}

		got := getResp.Note
		if got == nil {
			rt.Fatal("GetNote returned nil Note")
		}
		if got.FilePath != created.FilePath {
			rt.Fatalf("get: expected file_path %q, got %q", created.FilePath, got.FilePath)
		}
		if got.Title != title {
			rt.Fatalf("get: expected title %q, got %q", title, got.Title)
		}
		if got.Content != content {
			rt.Fatalf("get: expected content %q, got %q", content, got.Content)
		}
		if got.UpdatedAt == 0 {
			rt.Fatal("get: updated_at should be non-zero")
		}
	})
}

// Feature: api-unification, Property 2: UpdateNote returns full Note with updated content
// For any existing note and any new content string, calling UpdateNote should return
// an UpdateNoteResponse containing a Note whose content matches the new content,
// whose file_path and title match the original note, and whose updated_at is non-zero.
// **Validates: Requirements 3.3, 3.6, 4.2**
func TestProperty2_UpdateNoteReturnsFullNote(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewNotesServer(tmp, nopLogger())

		title := nameGen().Draw(rt, "title")
		origContent := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(rt, "origContent")
		newContent := rapid.StringMatching(`[a-zA-Z0-9 ]{0,100}`).Draw(rt, "newContent")

		createResp, err := srv.CreateNote(context.Background(), &pb.CreateNoteRequest{
			Title:   title,
			Content: origContent,
		})
		if err != nil {
			rt.Fatalf("CreateNote failed: %v", err)
		}

		filePath := createResp.Note.FilePath

		updateResp, err := srv.UpdateNote(context.Background(), &pb.UpdateNoteRequest{
			Id: filePath,
			Content:  newContent,
		})
		if err != nil {
			rt.Fatalf("UpdateNote failed: %v", err)
		}

		updated := updateResp.Note
		if updated == nil {
			rt.Fatal("UpdateNote returned nil Note")
		}
		if updated.FilePath != filePath {
			rt.Fatalf("expected file_path %q, got %q", filePath, updated.FilePath)
		}
		if updated.Title != title {
			rt.Fatalf("expected title %q, got %q", title, updated.Title)
		}
		if updated.Content != newContent {
			rt.Fatalf("expected content %q, got %q", newContent, updated.Content)
		}
		if updated.UpdatedAt == 0 {
			rt.Fatal("updated_at should be non-zero")
		}
	})
}
