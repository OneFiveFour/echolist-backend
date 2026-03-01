package server

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

	if resp.Note.FilePath != "Work/2026/note_Meeting.md" {
		t.Fatalf("unexpected file_path: %s", resp.Note.FilePath)
	}

	if resp.Note.Title != "Meeting" {
		t.Fatalf("unexpected title: %s", resp.Note.Title)
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

	if msg := connectErr.Message(); msg != "title must not be empty" {
		t.Fatalf("expected message 'title must not be empty', got %q", msg)
	}
}
