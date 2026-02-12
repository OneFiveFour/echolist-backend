package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	pb "notes-backend/proto/gen/notes/v1"
)

func (s *NotesServer) ListNotes(
	ctx context.Context,
	req *pb.ListNotesRequest,
) (*pb.ListNotesResponse, error) {

	var notes []*pb.Note

	root := s.dataDir
	if req.Path != "" {
		root = filepath.Join(s.dataDir, req.Path)
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// nur .md Dateien
		if filepath.Ext(info.Name()) != ".md" {
			return nil
		}

		relPath, _ := filepath.Rel(s.dataDir, path)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		note := &pb.Note{
			FilePath:  relPath,
			Title:     info.Name()[0 : len(info.Name())-3], // .md entfernen
			Content:   string(content),
			UpdatedAt: info.ModTime().UnixMilli(),
		}

		notes = append(notes, note)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &pb.ListNotesResponse{Notes: notes}, nil
}
