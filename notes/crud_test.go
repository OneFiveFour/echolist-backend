package notes_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/common"
	"echolist-backend/database"
	"echolist-backend/notes"
	pb "echolist-backend/proto/gen/notes/v1"
)

func TestCreateNote_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	resp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "Meeting",
		Content:   "# Meeting\nHello World",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	note := resp.Note
	if note.Id == "" {
		t.Fatal("expected non-empty Note.Id")
	}
	if err := common.ValidateUuidV4(note.Id); err != nil {
		t.Fatalf("invalid UUIDv4: %v", err)
	}
	if note.Title != "Meeting" {
		t.Fatalf("expected title 'Meeting', got %q", note.Title)
	}
	if note.Content != "# Meeting\nHello World" {
		t.Fatalf("expected content '# Meeting\\nHello World', got %q", note.Content)
	}
	if note.UpdatedAt <= 0 {
		t.Fatalf("expected UpdatedAt > 0, got %d", note.UpdatedAt)
	}
}

func TestCreateNote_FileExistsOnDisk(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	resp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "DiskCheck",
		Content:   "file content here",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	note := resp.Note
	notePath := database.NotePath("", note.Title, note.Id)
	absPath := filepath.Join(dataDir, notePath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", absPath, err)
	}
	if string(data) != "file content here" {
		t.Fatalf("file content mismatch: got %q, want %q", string(data), "file content here")
	}
}

func TestGetNote_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "GetTest",
		Content:   "some content",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	getResp, err := srv.GetNote(ctx, &pb.GetNoteRequest{
		Id: createResp.Note.Id,
	})
	if err != nil {
		t.Fatalf("GetNote failed: %v", err)
	}

	got := getResp.Note
	if got.Title != "GetTest" {
		t.Fatalf("title mismatch: got %q, want %q", got.Title, "GetTest")
	}
	if got.Content != "some content" {
		t.Fatalf("content mismatch: got %q, want %q", got.Content, "some content")
	}
}

func TestGetNote_NotFound(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.GetNote(ctx, &pb.GetNoteRequest{
		Id: "00000000-0000-4000-a000-000000000000",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestUpdateNote_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "Old",
		Content:   "old content",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	updateResp, err := srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
		Id:      createResp.Note.Id,
		Title:   "New",
		Content: "new content",
	})
	if err != nil {
		t.Fatalf("UpdateNote failed: %v", err)
	}

	got := updateResp.Note
	if got.Id != createResp.Note.Id {
		t.Fatalf("Id mismatch: got %q, want %q", got.Id, createResp.Note.Id)
	}
	if got.Title != "New" {
		t.Fatalf("title mismatch: got %q, want %q", got.Title, "New")
	}
	if got.Content != "new content" {
		t.Fatalf("content mismatch: got %q, want %q", got.Content, "new content")
	}
}

func TestUpdateNote_FileRenamed(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "Original",
		Content:   "original content",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	id := createResp.Note.Id

	_, err = srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
		Id:      id,
		Title:   "Renamed",
		Content: "renamed content",
	})
	if err != nil {
		t.Fatalf("UpdateNote failed: %v", err)
	}

	// Old file should no longer exist
	oldPath := database.NotePath("", "Original", id)
	oldAbsPath := filepath.Join(dataDir, oldPath)
	if _, err := os.Stat(oldAbsPath); !os.IsNotExist(err) {
		t.Fatalf("expected old file to be removed, but it still exists at %s", oldAbsPath)
	}

	// New file should exist with the new content
	newPath := database.NotePath("", "Renamed", id)
	newAbsPath := filepath.Join(dataDir, newPath)
	data, err := os.ReadFile(newAbsPath)
	if err != nil {
		t.Fatalf("expected new file at %s, got error: %v", newAbsPath, err)
	}
	if string(data) != "renamed content" {
		t.Fatalf("new file content mismatch: got %q, want %q", string(data), "renamed content")
	}
}

func TestDeleteNote_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "ToDelete",
		Content:   "delete me",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{
		Id: createResp.Note.Id,
	})
	if err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	// Verify GetNote returns NotFound
	_, err = srv.GetNote(ctx, &pb.GetNoteRequest{
		Id: createResp.Note.Id,
	})
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestDeleteNote_FileRemoved(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "FileGone",
		Content:   "will be removed",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	id := createResp.Note.Id
	notePath := database.NotePath("", "FileGone", id)
	absPath := filepath.Join(dataDir, notePath)

	// Verify file exists before delete
	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("expected file to exist before delete: %v", err)
	}

	_, err = srv.DeleteNote(ctx, &pb.DeleteNoteRequest{Id: id})
	if err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	// Verify file no longer exists
	if _, err := os.Stat(absPath); !os.IsNotExist(err) {
		t.Fatalf("expected file to be removed after delete, but stat returned: %v", err)
	}
}

func TestListNotes_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	titles := []string{"Note A", "Note B", "Note C"}
	contents := []string{"Content A", "Content B", "Content C"}
	for i, title := range titles {
		_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
			ParentDir: "",
			Title:     title,
			Content:   contents[i],
		})
		if err != nil {
			t.Fatalf("CreateNote(%q) failed: %v", title, err)
		}
	}

	listResp, err := srv.ListNotes(ctx, &pb.ListNotesRequest{
		ParentDir: "",
	})
	if err != nil {
		t.Fatalf("ListNotes failed: %v", err)
	}

	if len(listResp.Notes) != 3 {
		t.Fatalf("expected 3 notes, got %d", len(listResp.Notes))
	}

	// Verify all titles and contents are present
	found := make(map[string]string)
	for _, n := range listResp.Notes {
		found[n.Title] = n.Content
	}
	for i, title := range titles {
		content, ok := found[title]
		if !ok {
			t.Fatalf("expected note with title %q in list", title)
		}
		if content != contents[i] {
			t.Fatalf("note %q: content mismatch: got %q, want %q", title, content, contents[i])
		}
	}
}

func TestListNotes_FiltersByParentDir(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	// Pre-create directories
	if err := os.MkdirAll(filepath.Join(dataDir, "Work"), 0o755); err != nil {
		t.Fatalf("failed to create Work dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "Personal"), 0o755); err != nil {
		t.Fatalf("failed to create Personal dir: %v", err)
	}

	// Create notes in different dirs
	_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "Work",
		Title:     "Work Note 1",
		Content:   "work 1",
	})
	if err != nil {
		t.Fatalf("CreateNote in Work failed: %v", err)
	}
	_, err = srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "Work",
		Title:     "Work Note 2",
		Content:   "work 2",
	})
	if err != nil {
		t.Fatalf("CreateNote in Work failed: %v", err)
	}
	_, err = srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "Personal",
		Title:     "Personal Note",
		Content:   "personal",
	})
	if err != nil {
		t.Fatalf("CreateNote in Personal failed: %v", err)
	}

	// List only Work
	listResp, err := srv.ListNotes(ctx, &pb.ListNotesRequest{
		ParentDir: "Work",
	})
	if err != nil {
		t.Fatalf("ListNotes(Work) failed: %v", err)
	}
	if len(listResp.Notes) != 2 {
		t.Fatalf("expected 2 notes in Work, got %d", len(listResp.Notes))
	}

	// List only Personal
	listResp, err = srv.ListNotes(ctx, &pb.ListNotesRequest{
		ParentDir: "Personal",
	})
	if err != nil {
		t.Fatalf("ListNotes(Personal) failed: %v", err)
	}
	if len(listResp.Notes) != 1 {
		t.Fatalf("expected 1 note in Personal, got %d", len(listResp.Notes))
	}
}
