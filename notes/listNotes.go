package notes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) ListNotes(
	ctx context.Context,
	req *pb.ListNotesRequest,
) (*pb.ListNotesResponse, error) {

	root, err := pathutil.ValidateParentDir(s.dataDir, req.GetParentDir())
	if err != nil {
		return nil, err
	}

	dirEntries, err := os.ReadDir(root)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read directory: %w", err))
	}

	var notes []*pb.Note

	prefix := req.GetParentDir()
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}

		name := e.Name()

		if filepath.Ext(name) != ".md" || !strings.HasPrefix(name, "note_") {
			continue
		}

		entryPath := prefix + name

		fullPath := filepath.Join(root, name)
		info, err := e.Info()
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat %s: %w", fullPath, err))
		}

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read %s: %w", fullPath, err))
		}

		title, err := ExtractNoteTitle(name)
		if err != nil {
			continue
		}

		notes = append(notes, &pb.Note{
			FilePath:  entryPath,
			Title:     title,
			Content:   string(content),
			UpdatedAt: info.ModTime().UnixMilli(),
		})
	}

	return &pb.ListNotesResponse{Notes: notes}, nil
}
