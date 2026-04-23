package notes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	pb "echolist-backend/proto/gen/notes/v1"
)

func TestCreateNote_CreatesMarkdownFile(t *testing.T) {
	dataDir := t.TempDir()

	server := &NotesServer{
		dataDir: dataDir,
	}

	// Pre-create the parent directory (CreateNote no longer auto-creates intermediates)
	if err := os.MkdirAll(filepath.Join(dataDir, "Work", "2026"), 0755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}

	req := &pb.CreateNoteRequest{
		ParentDir: "Work/2026",
		Title:   "Meeting",
		Content: "# Meeting\n\nHello World",
	}

	resp, err := server.CreateNote(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	expectedPath := filepath.Join(
		dataDir,
		"Work",
		"2026",
		"note_Meeting.md",
	)

	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected file to exist at %s", expectedPath)
	}

	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != req.Content {
		t.Fatalf("file content mismatch")
	}

	if resp.Note.Title != "Meeting" {
		t.Fatalf("unexpected title: %s", resp.Note.Title)
	}

	if resp.Note.UpdatedAt <= 0 {
		t.Fatalf("expected positive UpdatedAt, got %d", resp.Note.UpdatedAt)
	}

	// UpdatedAt should reflect the file's actual mtime, not a synthetic timestamp
	info, err := os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("failed to stat created file: %v", err)
	}
	if resp.Note.UpdatedAt != info.ModTime().UnixMilli() {
		t.Fatalf("UpdatedAt %d does not match file mtime %d", resp.Note.UpdatedAt, info.ModTime().UnixMilli())
	}
}

func TestCreateNote_EmptyTitleRejected(t *testing.T) {
	dataDir := t.TempDir()

	server := &NotesServer{
		dataDir: dataDir,
	}

	req := &pb.CreateNoteRequest{
		ParentDir: "Work",
		Title:   "",
		Content: "some content",
	}

	_, err := server.CreateNote(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T: %v", err, err)
	}

	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connectErr.Code())
	}

	if msg := connectErr.Message(); msg != "name must not be empty" {
		t.Fatalf("expected message 'name must not be empty', got %q", msg)
	}
}

func TestCreateNote_NonExistentParentDirRejected(t *testing.T) {
	dataDir := t.TempDir()
	server := &NotesServer{dataDir: dataDir}

	req := &pb.CreateNoteRequest{
		ParentDir: "nonexistent/deep/path",
		Title:     "Test",
		Content:   "content",
	}

	_, err := server.CreateNote(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent parent dir, got nil")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T: %v", err, err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connectErr.Code())
	}

	// Verify no directories were created
	if _, err := os.Stat(filepath.Join(dataDir, "nonexistent")); !os.IsNotExist(err) {
		t.Fatal("directory should not have been created")
	}
}

func TestCreateNote_ExistingParentDirSucceeds(t *testing.T) {
	dataDir := t.TempDir()
	server := &NotesServer{dataDir: dataDir}

	// Pre-create the parent directory
	parentDir := filepath.Join(dataDir, "existing")
	if err := os.Mkdir(parentDir, 0755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}

	req := &pb.CreateNoteRequest{
		ParentDir: "existing",
		Title:     "Test",
		Content:   "content",
	}

	resp, err := server.CreateNote(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}
	if resp.Note.Title != "Test" {
		t.Fatalf("unexpected title: %s", resp.Note.Title)
	}
}

