package notes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"echolist-backend/common"
	"echolist-backend/database"
	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) DeleteNote(
	ctx context.Context,
	req *pb.DeleteNoteRequest,
) (*pb.DeleteNoteResponse, error) {

	// Validate the id field
	if err := common.ValidateUuidV4(req.GetId()); err != nil {
		return nil, err
	}

	// Query DB for note metadata
	noteRow, err := s.db.GetNote(req.GetId())
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		s.logger.Error("failed to query note", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to query note: %w", err))
	}

	// Compute file path from metadata
	notePath := database.NotePath(noteRow.ParentDir, noteRow.Title, noteRow.Id)
	absPath := filepath.Join(s.dataDir, notePath)

	// Lock the file path
	unlockFile := s.locks.Lock(absPath)
	defer unlockFile()

	// Delete file from disk first
	err = os.Remove(absPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logger.Error("failed to delete note file", "id", req.GetId(), "path", notePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete note: %w", err))
	}
	// If file was missing (os.ErrNotExist), we still proceed to delete the DB row (cleanup orphan)

	// Delete DB row
	deleted, err := s.db.DeleteNote(req.GetId())
	if err != nil {
		// File already removed but DB delete failed — log the error
		s.logger.Error("failed to delete note from database after file removal", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete note metadata: %w", err))
	}
	if !deleted {
		// This shouldn't happen since we already found the row above, but handle it
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
	}

	return &pb.DeleteNoteResponse{}, nil
}
