package notes

import (
	"context"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
	"pgregory.net/rapid"
)

// Feature: note-stable-ids, Property 1: Create-then-get round trip
// For any valid note title and content, creating a note via CreateNote and then
// retrieving it via GetNote using the returned id should produce a Note
// with the same id, title, content, and file_path. The updated_at should be non-zero
// in both responses.
// **Validates: Requirements 1.1, 2.1, 4.1, 8.1, 8.2, 10.1**
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
		if created.Id == "" {
			rt.Fatal("create: id should be non-empty")
		}

		getResp, err := srv.GetNote(context.Background(), &pb.GetNoteRequest{
			Id: created.Id,
		})
		if err != nil {
			rt.Fatalf("GetNote failed: %v", err)
		}

		got := getResp.Note
		if got == nil {
			rt.Fatal("GetNote returned nil Note")
		}
		if got.Id != created.Id {
			rt.Fatalf("get: expected id %q, got %q", created.Id, got.Id)
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

// Feature: note-stable-ids, Property 2: UpdateNote returns full Note with updated content
// For any existing note and any new content string, calling UpdateNote should return
// an UpdateNoteResponse containing a Note whose content matches the new content,
// whose id, file_path and title match the original note, and whose updated_at is non-zero.
// **Validates: Requirements 1.4, 3.3, 5.1**
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

		noteId := createResp.Note.Id
		filePath := createResp.Note.FilePath

		updateResp, err := srv.UpdateNote(context.Background(), &pb.UpdateNoteRequest{
			Id:      noteId,
			Title:   title,
			Content: newContent,
		})
		if err != nil {
			rt.Fatalf("UpdateNote failed: %v", err)
		}

		updated := updateResp.Note
		if updated == nil {
			rt.Fatal("UpdateNote returned nil Note")
		}
		if updated.Id != noteId {
			rt.Fatalf("expected id %q, got %q", noteId, updated.Id)
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
