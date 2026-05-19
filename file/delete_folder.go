package file

import (
	"context"
	"fmt"
	"os"

	"connectrpc.com/connect"

	"echolist-backend/common"
	filev1 "echolist-backend/proto/gen/file/v1"
)

func (s *FileServer) DeleteFolder(
	ctx context.Context,
	req *filev1.DeleteFolderRequest,
) (*filev1.DeleteFolderResponse, error) {
	// folder_path must not be empty (can't delete root)
	folderPath := req.GetFolderPath()
	if folderPath == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder_path must not be empty"))
	}

	target, err := common.ValidatePath(s.dataDir, folderPath)
	if err != nil {
		return nil, err
	}

	// Check folder exists and is a directory
	err = common.RequireDir(target, "folder")
	if err != nil {
		return nil, err
	}

	unlock := s.locks.Lock(target)
	defer unlock()

	// Delete notes and task_lists rows from SQLite before removing the folder.
	err = s.db.DeleteByParentDir(folderPath)
	if err != nil {
		s.logger.Error("failed to delete DB rows for folder", "path", folderPath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete database entries: %w", err))
	}

	// Remove folder and all contents
	err = os.RemoveAll(target)
	if err != nil {
		s.logger.Warn("failed to delete folder from disk after DB cascade", "path", folderPath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete folder: %w", err))
	}
	
	return &filev1.DeleteFolderResponse{}, nil
}
