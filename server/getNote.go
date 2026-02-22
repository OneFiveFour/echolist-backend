package server

import (
	"context"
	"os"
	"path/filepath"

	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) GetNote(
	ctx context.Context,
	req *pb.GetNoteRequest,
) (*pb.GetNoteResponse, error) {

	fullPath := filepath.Join(s.dataDir, req.FilePath)

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	return &pb.GetNoteResponse{
		FilePath:  req.FilePath,
		Title:     info.Name()[:len(info.Name())-3],
		Content:   string(content),
		UpdatedAt: info.ModTime().UnixMilli(),
	}, nil
}
