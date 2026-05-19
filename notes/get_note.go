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

func (s *NotesServer) GetNote(
	ctx context.Context,
	req *pb.GetNoteRequest,
) (*pb.GetNoteResponse, error) {

	// Validate the id field
	id := req.GetId()
	err := common.ValidateUuidV4(id)
	if err != nil {
		return nil, err
	}

	// Query DB for note metadata
	noteRow, err := s.db.GetNote(id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		s.logger.Error("failed to query note", "id", id, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to query note: %w", err))
	}

	// Compute file path from metadata
	notePath := NotePath(noteRow.ParentDir, noteRow.Title, noteRow.Id)
	absPath := filepath.Join(s.dataDir, notePath)

	// Read content from disk
	content, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		s.logger.Error("failed to read note", "path", notePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read note: %w", err))
	}

	note := &pb.Note{
		Id:        noteRow.Id,
		Title:     noteRow.Title,
		Content:   string(content),
		UpdatedAt: noteRow.UpdatedAt,
		ParentDir: noteRow.ParentDir,
	}

	return &pb.GetNoteResponse{Note: note}, nil
}
