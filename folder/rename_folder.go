package folder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	folderv1 "echolist-backend/proto/gen/folder/v1"
)

func (s *FolderServer) RenameFolder(
	ctx context.Context,
	req *folderv1.RenameFolderRequest,
) (*folderv1.RenameFolderResponse, error) {
	// Validate new name
	if err := validateName(req.GetNewName()); err != nil {
		return nil, err
	}

	// folder_path must not be empty (can't rename root)
	if req.GetFolderPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder_path must not be empty"))
	}

	oldPath := filepath.Clean(filepath.Join(s.dataDir, req.GetFolderPath()))

	// Ensure old path is within the data directory
	if !pathutil.IsSubPath(s.dataDir, oldPath) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder path escapes data directory"))
	}

	// Check folder exists and is a directory
	info, err := os.Stat(oldPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("folder does not exist"))
	}
	if !info.IsDir() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("path is not a directory"))
	}

	parentDir := filepath.Dir(oldPath)

	// Check case-insensitive sibling duplicates (excluding the folder being renamed)
	existing, err := os.ReadDir(parentDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read parent directory: %w", err))
	}
	oldBase := filepath.Base(oldPath)
	for _, e := range existing {
		if strings.EqualFold(e.Name(), req.GetNewName()) && !strings.EqualFold(e.Name(), oldBase) {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("a folder or file with that name already exists (case-insensitive)"))
		}
	}

	// Rename
	newPath := filepath.Join(parentDir, req.GetNewName())
	if err := os.Rename(oldPath, newPath); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to rename folder: %w", err))
	}

	// Return parent listing
	entries, err := listDirectory(parentDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &folderv1.RenameFolderResponse{Entries: entries}, nil
}
