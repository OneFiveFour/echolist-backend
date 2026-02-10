package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pb "notes-backend/gen/notes"
)

func (s *NotesServer) UpdateNote(
	ctx context.Context,
	req *pb.UpdateNoteRequest,
) (*pb.UpdateNoteResponse, error) {

	fullPath := filepath.Join(DataDir, req.FilePath)

	err := atomicWriteFile(fullPath, []byte(req.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to update note: %w", err)
	}

	return &pb.UpdateNoteResponse{
		UpdatedAt: time.Now().UnixMilli(),
	}, nil
}

