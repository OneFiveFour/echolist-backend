package folder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	folderv1 "notes-backend/proto/gen/folder/v1"
)

func (s *FolderServer) DeleteFolder(
	ctx context.Context,
	req *folderv1.DeleteFolderRequest,
) (*folderv1.DeleteFolderResponse, error) {
	// folder_path must not be empty (can't delete root)
	if req.GetFolderPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder_path must not be empty"))
	}

	domainRoot := filepath.Join(s.dataDir, req.GetDomain())
	target := filepath.Clean(filepath.Join(domainRoot, req.GetFolderPath()))

	// Ensure target is within domain root
	if !isSubPath(domainRoot, target) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder path escapes domain root"))
	}

	// Check folder exists and is a directory
	info, err := os.Stat(target)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("folder does not exist"))
	}
	if !info.IsDir() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("path is not a directory"))
	}

	parentDir := filepath.Dir(target)

	// Remove folder and all contents
	if err := os.RemoveAll(target); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete folder: %w", err))
	}

	// Return parent listing
	entries, err := listDirectory(parentDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &folderv1.DeleteFolderResponse{Entries: entries}, nil
}
