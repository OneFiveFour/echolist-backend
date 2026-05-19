package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"echolist-backend/common"
	filev1 "echolist-backend/proto/gen/file/v1"
)

func (s *FileServer) CreateFolder(
	ctx context.Context,
	req *filev1.CreateFolderRequest,
) (*filev1.CreateFolderResponse, error) {
	// Validate folder name
	name := req.GetName()
	err := common.ValidateName(name)
	if err != nil {
		return nil, err
	}

	// Validate parent directory path
	parentDirRel := req.GetParentDir()
	parentDir, err := common.ValidateParentDir(s.dataDir, parentDirRel)
	if err != nil {
		return nil, err
	}

	// Validate parent directory exists
	err = common.RequireDir(parentDir, "parent directory")
	if err != nil {
		return nil, err
	}

	// Check for duplicate (case-sensitive)
	newDir := filepath.Join(parentDir, name)

	unlock := s.locks.Lock(newDir)
	defer unlock()

	if _, err := os.Stat(newDir); err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("a folder or file with that name already exists"))
	}

	if err := os.Mkdir(newDir, 0755); err != nil {
		s.logger.Error("failed to create folder", "path", req.GetParentDir()+"/"+name, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create folder: %w", err))
	}

	// Build relative path for the created folder (with trailing /)
	relPath := filepath.Join(parentDirRel, name) + "/"
	return &filev1.CreateFolderResponse{
		Folder: &filev1.Folder{
			Path: relPath,
			Name: name,
		},
	}, nil
}
