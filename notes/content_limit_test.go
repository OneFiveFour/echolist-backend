package notes

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/notes/v1"
)

func TestCreateNote_ContentTooLarge(t *testing.T) {
	dataDir := t.TempDir()
	server := NewNotesServer(dataDir, nopLogger())

	oversized := strings.Repeat("x", pathutil.MaxNoteContentBytes+1)

	_, err := server.CreateNote(context.Background(), &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "big",
		Content:   oversized,
	})
	if err == nil {
		t.Fatal("expected error for oversized content, got nil")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

func TestCreateNote_ContentAtLimit(t *testing.T) {
	dataDir := t.TempDir()
	server := NewNotesServer(dataDir, nopLogger())

	content := strings.Repeat("x", pathutil.MaxNoteContentBytes)

	resp, err := server.CreateNote(context.Background(), &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "maxnote",
		Content:   content,
	})
	if err != nil {
		t.Fatalf("content at exact limit should succeed: %v", err)
	}
	if resp.Note.Title != "maxnote" {
		t.Fatalf("unexpected title: %s", resp.Note.Title)
	}
}

func TestUpdateNote_ContentTooLarge(t *testing.T) {
	dataDir := t.TempDir()
	server := NewNotesServer(dataDir, nopLogger())

	// Create a note file to update
	notePath := filepath.Join(dataDir, "note_test.md")
	if err := os.WriteFile(notePath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	oversized := strings.Repeat("x", pathutil.MaxNoteContentBytes+1)

	_, err := server.UpdateNote(context.Background(), &pb.UpdateNoteRequest{
		FilePath: "note_test.md",
		Content:  oversized,
	})
	if err == nil {
		t.Fatal("expected error for oversized content, got nil")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}

	// Verify original file was not modified
	data, _ := os.ReadFile(notePath)
	if string(data) != "old" {
		t.Fatal("file should not have been modified")
	}
}

func TestCreateNote_TitleTooLong(t *testing.T) {
	dataDir := t.TempDir()
	server := NewNotesServer(dataDir, nopLogger())

	longTitle := strings.Repeat("a", pathutil.MaxNameLen+1)

	_, err := server.CreateNote(context.Background(), &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     longTitle,
		Content:   "short",
	})
	if err == nil {
		t.Fatal("expected error for oversized title, got nil")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}
