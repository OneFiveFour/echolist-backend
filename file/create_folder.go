package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	filev1 "echolist-backend/proto/gen/file/v1"
)

func (s *FileServer) CreateFolder(
	ctx context.Context,
	req *filev1.CreateFolderRequest,
) (*filev1.CreateFolderResponse, error) {
	if err := validateName(req.GetName()); err != nil {
		return nil, err
	}

	// Resolve parent directory
	parentDir := filepath.Join(s.dataDir, req.GetParentPath())
	parentDir = filepath.Clean(parentDir)

	// Ensure parent is within the data directory (or is the root itself)
	if parentDir != s.dataDir && !pathutil.IsSubPath(s.dataDir, parentDir) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parent path escapes data directory"))
	}

	// Check parent exists
	info, err := os.Stat(parentDir)
	if err != nil || !info.IsDir() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("parent directory does not exist"))
	}

	// Check case-insensitive duplicates
	existing, err := os.ReadDir(parentDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read parent directory: %w", err))
	}
	for _, e := range existing {
		if strings.EqualFold(e.Name(), req.GetName()) {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("a folder or file with that name already exists (case-insensitive)"))
		}
	}

	// Create the folder
	newDir := filepath.Join(parentDir, req.GetName())
	if err := os.Mkdir(newDir, 0755); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create folder: %w", err))
	}

	// Build relative path for the created folder (with trailing /)
	relPath := filepath.Join(req.GetParentPath(), req.GetName()) + "/"
	return &filev1.CreateFolderResponse{
		Folder: &filev1.Folder{
			Path: relPath,
			Name: req.GetName(),
		},
	}, nil
}
