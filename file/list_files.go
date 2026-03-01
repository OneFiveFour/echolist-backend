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

func (s *FileServer) ListFiles(
	ctx context.Context,
	req *filev1.ListFilesRequest,
) (*filev1.ListFilesResponse, error) {

	parentDir := filepath.Clean(filepath.Join(s.dataDir, req.GetParentDir()))
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
		} else if !strings.HasPrefix(name, "note_") && !strings.HasPrefix(name, "tasks_") {
			continue
		}
		result = append(result, name)
	}
	
	return &filev1.ListFilesResponse{Entries: result}, nil
}
