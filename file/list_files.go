package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/common"
	filev1 "echolist-backend/proto/gen/file/v1"
)

// matchesFileType returns true if name has both the correct prefix and suffix
// for the given file type, with at least one character between them.
func matchesFileType(name string, ft common.FileType) bool {
	return strings.HasPrefix(name, ft.Prefix) &&
		filepath.Ext(name) == ft.Suffix &&
		len(name) > len(ft.Prefix)+len(ft.Suffix)
}

func (s *FileServer) ListFiles(
	ctx context.Context,
	req *filev1.ListFilesRequest,
) (*filev1.ListFilesResponse, error) {

	parentDir, err := common.ValidateParentDir(s.dataDir, req.GetParentDir())
	if err != nil {
		return nil, err
	}

	if err := common.RequireDir(parentDir, "parent directory"); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		s.logger.Error("failed to read directory", "path", req.GetParentDir(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read directory: %w", err))
	}

	var result []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		} else if !matchesFileType(name, common.NoteFileType) && !matchesFileType(name, common.TaskListFileType) {
			continue
		}
		result = append(result, name)
	}
	
	return &filev1.ListFilesResponse{Entries: result}, nil
}
