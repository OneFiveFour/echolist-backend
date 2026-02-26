package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) UpdateNote(
	ctx context.Context,
	req *pb.UpdateNoteRequest,
) (*pb.UpdateNoteResponse, error) {

	fullPath := filepath.Join(s.dataDir, req.FilePath)

	err := atomicWriteFile(fullPath, []byte(req.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to update note: %w", err)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat note after update: %w", err)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read note after update: %w", err)
	}

	note := &pb.Note{
		FilePath:  req.FilePath,
		Title:     strings.TrimPrefix(info.Name()[:len(info.Name())-3], "note_"),
		Content:   string(content),
		UpdatedAt: info.ModTime().UnixMilli(),
	}

	return &pb.UpdateNoteResponse{Note: note}, nil
}
