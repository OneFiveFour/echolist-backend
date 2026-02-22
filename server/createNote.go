package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) CreateNote(
	ctx context.Context,
	req *pb.CreateNoteRequest,
) (*pb.CreateNoteResponse, error) {

	destination := filepath.Join(s.dataDir, req.Path)

	err := os.MkdirAll(destination, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	relativeFilePath := filepath.Join(req.Path, req.Title+".md")
	absoluteFilePath := filepath.Join(s.dataDir, relativeFilePath)

	err = atomicWriteFile(absoluteFilePath, []byte(req.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to write note: %w", err)
	}

	resp := &pb.CreateNoteResponse{
		FilePath:  relativeFilePath,
		Title:     req.Title,
		Content:   req.Content,
		UpdatedAt: time.Now().UnixMilli(),
	}

	return resp, nil

}
