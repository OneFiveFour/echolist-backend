package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pb "notes-backend/gen/notes"
)

const DataDir = "./data" // Docker volume mount

func (s *NotesServer) CreateNote(
	ctx context.Context,
	req *pb.CreateNoteRequest,
) (*pb.CreateNoteResponse, error) {

	// Path of new note
	folderPath := filepath.Join(DataDir, req.Path)

	// Create folder if needed
	err := os.MkdirAll(folderPath, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	fileName := filepath.Join(folderPath, req.Title+".md")

	// Write file
	err := atomicWriteFile(fullPath, []byte(req.Content))
	if err != nil {
		return nil, fmt.Errorf("failed to write note: %w", err)
	}

	filePath := filepath.Join(req.Path, req.Title)

	resp := &pb.CreateNoteResponse{
		FilePath:        filePath,
		Title:     req.Title,
		Content: req.Content,
		UpdatedAt: time.Now().UnixMilli(),
	}

	return resp, nil

}
