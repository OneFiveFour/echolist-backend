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

func (s *FileServer) UpdateFolder(
	ctx context.Context,
	req *filev1.UpdateFolderRequest,
) (*filev1.UpdateFolderResponse, error) {
	if err := validateName(req.GetNewName()); err != nil {
		return nil, err
	}
	if req.GetFolderPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder_path must not be empty"))
	}
	oldPath := filepath.Clean(filepath.Join(s.dataDir, req.GetFolderPath()))
	if !pathutil.IsSubPath(s.dataDir, oldPath) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder path escapes data directory"))
	}
	info, err := os.Stat(oldPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("folder does not exist"))
	}
	if !info.IsDir() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("path is not a directory"))
	}
	parentDir := filepath.Dir(oldPath)
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
	newPath := filepath.Join(parentDir, req.GetNewName())
	if err := os.Rename(oldPath, newPath); err != nil {
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
