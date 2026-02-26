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

func (s *FolderServer) GetFolder(
	ctx context.Context,
	req *folderv1.GetFolderRequest,
) (*folderv1.GetFolderResponse, error) {
	if req.GetFolderPath() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder_path must not be empty"))
	}

	target := filepath.Clean(filepath.Join(s.dataDir, req.GetFolderPath()))

	if !pathutil.IsSubPath(s.dataDir, target) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("folder path escapes data directory"))
	}

	info, err := os.Stat(target)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("folder does not exist"))
	}
	if !info.IsDir() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("path is not a directory"))
	}

	return &folderv1.GetFolderResponse{
		Folder: &folderv1.Folder{
			Path: req.GetFolderPath(),
			Name: filepath.Base(target),
		},
	}, nil
}
