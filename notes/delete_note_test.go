package notes

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "echolist-backend/proto/gen/notes/v1"
)

func TestDeleteNote(t *testing.T) {
	tmp := t.TempDir()
	s := NewNotesServer(tmp, nopLogger())

	// Create a note via the RPC to get a valid ID
	createResp, err := s.CreateNote(context.Background(), &pb.CreateNoteRequest{
		Title:   "todelete",
		Content: "bye",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	noteId := createResp.Note.Id
	notePath := filepath.Join(tmp, createResp.Note.FilePath)

	_, err = s.DeleteNote(context.Background(), &pb.DeleteNoteRequest{Id: noteId})
	if err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	if _, err := os.Stat(notePath); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, stat error: %v", err)
	}
}
