package notes_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/notes"
	pb "echolist-backend/proto/gen/notes/v1"
)

func TestCreateNote_EmptyTitleRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "",
		Content:   "some content",
	})
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateNote_NullByteTitleRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "bad\x00title",
		Content:   "some content",
	})
	if err == nil {
		t.Fatal("expected error for null byte in title, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateNote_PathTraversalRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "../etc",
		Title:     "Legit",
		Content:   "some content",
	})
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	code := connect.CodeOf(err)
	if code != connect.CodeInvalidArgument && code != connect.CodeNotFound {
		t.Fatalf("expected CodeInvalidArgument or CodeNotFound, got %v", code)
	}
}

func TestGetNote_InvalidUUIDRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.GetNote(ctx, &pb.GetNoteRequest{
		Id: "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestUpdateNote_InvalidUUIDRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
		Id:      "not-a-uuid",
		Title:   "Valid Title",
		Content: "some content",
	})
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestDeleteNote_InvalidUUIDRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.DeleteNote(ctx, &pb.DeleteNoteRequest{
		Id: "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateNote_ContentExceedsLimitRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	// 1 MiB = 1048576 bytes; exceed by 1
	bigContent := strings.Repeat("x", 1048576+1)

	_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "Big Note",
		Content:   bigContent,
	})
	if err == nil {
		t.Fatal("expected error for content exceeding limit, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateNote_TitleWithPathSeparatorRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "",
		Title:     "bad/title",
		Content:   "some content",
	})
	if err == nil {
		t.Fatal("expected error for title with path separator, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateNote_NonExistentParentDirRejected(t *testing.T) {
	dataDir := t.TempDir()
	srv := notes.NewNotesServer(dataDir, notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	// Ensure the directory does NOT exist
	dirPath := filepath.Join(dataDir, "nonexistent", "path")
	if _, err := os.Stat(dirPath); err == nil {
		t.Fatal("expected directory to not exist")
	}

	_, err := srv.CreateNote(ctx, &pb.CreateNoteRequest{
		ParentDir: "nonexistent/path",
		Title:     "Orphan",
		Content:   "some content",
	})
	if err == nil {
		t.Fatal("expected error for non-existent parent dir, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestUpdateNote_NonExistentNoteRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.UpdateNote(ctx, &pb.UpdateNoteRequest{
		Id:      "00000000-0000-4000-a000-000000000000",
		Title:   "Updated Title",
		Content: "updated content",
	})
	if err == nil {
		t.Fatal("expected error for non-existent note, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestDeleteNote_NonExistentNoteRejected(t *testing.T) {
	srv := notes.NewNotesServer(t.TempDir(), notes.NewTestDB(t), notes.NopLogger())
	ctx := context.Background()

	_, err := srv.DeleteNote(ctx, &pb.DeleteNoteRequest{
		Id: "00000000-0000-4000-a000-000000000000",
	})
	if err == nil {
		t.Fatal("expected error for non-existent note, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}
