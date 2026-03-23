package notes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/common"
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

	dirEntries, err := os.ReadDir(root)
	if err != nil {
		s.logger.Error("failed to read directory", "path", req.GetParentDir(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read directory: %w", err))
	}

	var notes []*pb.Note

	prefix := parentDir
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	for _, e := range dirEntries {
		if e.IsDir() {
			continue
		}

		name := e.Name()

		if filepath.Ext(name) != common.NoteFileType.Suffix || !strings.HasPrefix(name, common.NoteFileType.Prefix) {
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

	// Read registry and build reverse map (filePath → id)
	regPath := registryPath(s.dataDir)
	registry, err := registryRead(regPath)
	if err != nil {
		s.logger.Error("failed to read registry", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read registry: %w", err))
	}

	reverseMap := make(map[string]string, len(registry))
	for id, fp := range registry {
		reverseMap[fp] = id
	}

	for _, n := range notes {
		n.Id = reverseMap[n.FilePath] // empty string if not found
	}

	return &pb.ListNotesResponse{Notes: notes}, nil
}
