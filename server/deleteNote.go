package server

import (
	"context"
	"os"
	"path/filepath"

	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) DeleteNote(
	ctx context.Context,
	req *pb.DeleteNoteRequest,
) (*pb.DeleteNoteResponse, error) {

	fullPath := filepath.Join(s.dataDir, req.FilePath)

	if err := os.Remove(fullPath); err != nil {
		return nil, err
	}

	return &pb.DeleteNoteResponse{}, nil
}
