package folder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	folderv1 "echolist-backend/proto/gen/folder/v1"
)

func (s *FolderServer) ListFolders(
	ctx context.Context,
	req *folderv1.ListFoldersRequest,
) (*folderv1.ListFoldersResponse, error) {
	parentDir := filepath.Clean(filepath.Join(s.dataDir, req.GetParentPath()))

	if parentDir != s.dataDir && !pathutil.IsSubPath(s.dataDir, parentDir) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parent path escapes data directory"))
	}

	info, err := os.Stat(parentDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("parent directory does not exist"))
	}
	if !info.IsDir() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("parent path is not a directory"))
	}

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read directory: %w", err))
	}

	var folders []*folderv1.Folder
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		relPath := filepath.Join(req.GetParentPath(), e.Name()) + "/"
		folders = append(folders, &folderv1.Folder{
			Path: relPath,
			Name: e.Name(),
		})
	}

	return &folderv1.ListFoldersResponse{Folders: folders}, nil
}
