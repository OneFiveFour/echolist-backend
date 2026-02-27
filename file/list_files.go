package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	filev1 "echolist-backend/proto/gen/file/v1"
)

func (s *FileServer) ListFiles(
	ctx context.Context,
	req *filev1.ListFilesRequest,
) (*filev1.ListFilesResponse, error) {

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

	var result []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		result = append(result, name)
	}
	
	return &filev1.ListFilesResponse{Entries: result}, nil
}
