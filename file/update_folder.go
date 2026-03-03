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

func (s *FileServer) UpdateFolder(
	ctx context.Context,
	req *filev1.UpdateFolderRequest,
) (*filev1.UpdateFolderResponse, error) {
	if err := common.ValidateName(req.GetNewName()); err != nil {
		return nil, err
	}
	if req.GetFolderPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder_path must not be empty"))
	}
	oldPath, err := common.ValidatePath(s.dataDir, req.GetFolderPath())
	if err != nil {
		return nil, err
	}
	if err := common.RequireDir(oldPath, "folder"); err != nil {
		return nil, err
	}
	parentDir := filepath.Dir(oldPath)
	newPath := filepath.Join(parentDir, req.GetNewName())
	oldBase := filepath.Base(oldPath)

	unlock := s.locks.Lock(oldPath)
	defer unlock()

	// Check for exact duplicate sibling (case-sensitive)
	if req.GetNewName() != oldBase {
		if _, err := os.Stat(newPath); err == nil {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("a folder or file with that name already exists"))
		}
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		s.logger.Error("failed to rename folder", "path", req.GetFolderPath(), "newName", req.GetNewName(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to rename folder: %w", err))
	}
	relParent, err := filepath.Rel(s.dataDir, parentDir)
	if err != nil {
		relParent = ""
	}
	relPath := filepath.Join(relParent, req.GetNewName()) + "/"
	return &filev1.UpdateFolderResponse{
		Folder: &filev1.Folder{
			Path: relPath,
			Name: req.GetNewName(),
		},
	}, nil
}
