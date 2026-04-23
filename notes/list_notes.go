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

func (s *NotesServer) ListNotes(
	ctx context.Context,
	req *pb.ListNotesRequest,
) (*pb.ListNotesResponse, error) {

	parentDir := req.GetParentDir()

	root, err := common.ValidateParentDir(s.dataDir, parentDir)
	if err != nil {
		return nil, err
	}

	if err := common.RequireDir(root, "parent directory"); err != nil {
		return nil, err
	}

	// Query DB for note metadata in this directory
	noteRows, err := s.db.ListNotes(parentDir)
	if err != nil {
		s.logger.Error("failed to list notes from database", "parentDir", parentDir, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list notes: %w", err))
	}

	var notes []*pb.Note
	for _, row := range noteRows {
		// Compute file path from metadata
		notePath := database.NotePath(row.ParentDir, row.Title, row.Id)
		absPath := filepath.Join(s.dataDir, notePath)

		// Read content from disk
		content, err := os.ReadFile(absPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Skip notes whose files are missing — don't fail the listing
				s.logger.Warn("note file missing on disk, skipping", "id", row.Id, "path", notePath)
				continue
			}
			s.logger.Error("failed to read note file", "id", row.Id, "path", notePath, "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read note: %w", err))
		}

		notes = append(notes, &pb.Note{
			Id:        row.Id,
			Title:     row.Title,
			Content:   string(content),
			UpdatedAt: row.UpdatedAt,
		})
	}

	return &pb.ListNotesResponse{Notes: notes}, nil
}
