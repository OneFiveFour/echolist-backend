package file

import (
	"context"
	"fmt"
	"os"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	filev1 "echolist-backend/proto/gen/file/v1"
)

func (s *FileServer) ListFiles(
	ctx context.Context,
	req *filev1.ListFilesRequest,
) (*filev1.ListFilesResponse, error) {

	parentDir, err := pathutil.ValidateParentDir(s.dataDir, req.GetParentDir())
	if err != nil {
		return nil, err
	}

	if err := pathutil.RequireDir(parentDir, "parent directory"); err != nil {
		return nil, err
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
		} else if !strings.HasPrefix(name, pathutil.NoteFileType.Prefix) && !strings.HasPrefix(name, pathutil.TaskListFileType.Prefix) {
			continue
		}
		result = append(result, name)
	}
	
	return &filev1.ListFilesResponse{Entries: result}, nil
}
