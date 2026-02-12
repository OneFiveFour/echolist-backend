package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "notes-backend/proto/gen/notes/v1"
)

func TestCreateNote_CreatesMarkdownFile(t *testing.T) {
	dataDir := t.TempDir()

	server := &NotesServer{
		dataDir: dataDir,
	}

	req := &pb.CreateNoteRequest{
		Path:    "Work/2026",
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
		"Meeting.md",
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

	if resp.FilePath != "Work/2026/Meeting.md" {
		t.Fatalf("unexpected file_path: %s", resp.FilePath)
	}

	if resp.Title != "Meeting" {
		t.Fatalf("unexpected title: %s", resp.Title)
	}
}
