package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) ListNotes(
	ctx context.Context,
	req *pb.ListNotesRequest,
) (*pb.ListNotesResponse, error) {

	root := s.dataDir
	if req.Path != "" {
		root = filepath.Join(s.dataDir, req.Path)
	}

	dirEntries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var notes []*pb.Note
	var entries []string

	prefix := req.Path
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	for _, e := range dirEntries {
		name := e.Name()

		if e.IsDir() {
			entries = append(entries, prefix+name+"/")
			continue
		}

		if filepath.Ext(name) != ".md" {
			continue
		}

		entryPath := prefix + name
		entries = append(entries, entryPath)

		fullPath := filepath.Join(root, name)
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", fullPath, err)
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", fullPath, err)
		}

		notes = append(notes, &pb.Note{
			FilePath:  entryPath,
			Title:     strings.TrimSuffix(name, ".md"),
			Content:   string(content),
			UpdatedAt: info.ModTime().UnixMilli(),
		})
	}

	return &pb.ListNotesResponse{Notes: notes, Entries: entries}, nil
}
