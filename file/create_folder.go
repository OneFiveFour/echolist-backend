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
	parentDir, err := pathutil.ValidateParentDir(s.dataDir, req.GetParentDir())
	if err != nil {
		return nil, err
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
	relPath := filepath.Join(req.GetParentDir(), req.GetName()) + "/"
	return &filev1.CreateFolderResponse{
		Folder: &filev1.Folder{
			Path: relPath,
			Name: req.GetName(),
		},
	}, nil
}
