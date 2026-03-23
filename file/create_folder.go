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
	if err := common.ValidateName(req.GetName()); err != nil {
		return nil, err
	}

	parentDirRel := req.GetParentDir()

	parentDir, err := common.ValidateParentDir(s.dataDir, parentDirRel)
	if err != nil {
		return nil, err
	}

	if err := common.RequireDir(parentDir, "parent directory"); err != nil {
		return nil, err
	}

	// Check for duplicate (case-sensitive)
	newDir := filepath.Join(parentDir, req.GetName())

	unlock := s.locks.Lock(newDir)
	defer unlock()

	if _, err := os.Stat(newDir); err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("a folder or file with that name already exists"))
	}

	if err := os.Mkdir(newDir, 0755); err != nil {
		s.logger.Error("failed to create folder", "path", req.GetParentDir()+"/"+req.GetName(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create folder: %w", err))
	}

	// Build relative path for the created folder (with trailing /)
	relPath := filepath.Join(parentDirRel, req.GetName()) + "/"
	return &filev1.CreateFolderResponse{
		Folder: &filev1.Folder{
			Path: relPath,
			Name: req.GetName(),
		},
	}, nil
}
