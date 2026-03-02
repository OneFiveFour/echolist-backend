package file

import (
	"context"
	"fmt"
	"os"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	filev1 "echolist-backend/proto/gen/file/v1"
)

func (s *FileServer) DeleteFolder(
	ctx context.Context,
	req *filev1.DeleteFolderRequest,
) (*filev1.DeleteFolderResponse, error) {
	// folder_path must not be empty (can't delete root)
	if req.GetFolderPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder_path must not be empty"))
	}

	target, err := pathutil.ValidatePath(s.dataDir, req.GetFolderPath())
	if err != nil {
		return nil, err
	}

	// Check folder exists and is a directory
	if err := pathutil.RequireDir(target, "folder"); err != nil {
		return nil, err
	}

	unlock := s.locks.Lock(target)
	defer unlock()

	// Remove folder and all contents
	if err := os.RemoveAll(target); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete folder: %w", err))
	}
	
	return &filev1.DeleteFolderResponse{}, nil
}
