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

	folderPath := req.GetFolderPath()
	oldPath, err := common.ValidatePath(s.dataDir, folderPath)
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
		s.logger.Error("failed to rename folder", "path", folderPath, "newName", req.GetNewName(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to rename folder: %w", err))
	}

	// Compute the new relative path for the renamed folder.
	// The old relative path is folderPath (e.g. "Work" or "Projects/Docs").
	// The new relative path replaces the last segment with the new name.
	oldRelPath := folderPath
	parentRel := filepath.Dir(oldRelPath)
	newRelPath := req.GetNewName()
	if parentRel != "." {
		newRelPath = parentRel + "/" + req.GetNewName()
	}

	if err := s.db.RenameParentDir(oldRelPath, newRelPath); err != nil {
		// Rollback: rename folder back on disk.
		if rbErr := os.Rename(newPath, oldPath); rbErr != nil {
			s.logger.Error("failed to rollback folder rename", "newPath", newPath, "oldPath", oldPath, "error", rbErr)
		}
		s.logger.Error("failed to update DB parent dirs", "oldPath", oldRelPath, "newPath", newRelPath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update database entries: %w", err))
	}

	return &filev1.UpdateFolderResponse{
		Folder: &filev1.Folder{
			Path: newRelPath + "/",
			Name: req.GetNewName(),
		},
	}, nil
}
