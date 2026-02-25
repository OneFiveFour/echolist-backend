package folder

import (
	"fmt"
	"os"
	"strings"

	"connectrpc.com/connect"

	folderv1 "echolist-backend/proto/gen/folder/v1"
	"echolist-backend/proto/gen/folder/v1/folderv1connect"
)

// FolderServer implements the FolderService RPC interface.
type FolderServer struct {
	folderv1connect.UnimplementedFolderServiceHandler
	dataDir string
}

// NewFolderServer creates a new FolderServer rooted at dataDir.
func NewFolderServer(dataDir string) *FolderServer {
	return &FolderServer{dataDir: dataDir}
}


// validateName checks that a folder name is non-empty and contains no
// path separators, dots-only names, or null bytes.
func validateName(name string) error {
	if name == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not be empty"))
	}
	if strings.ContainsAny(name, "/\\") {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not contain path separators"))
	}
	if name == "." || name == ".." {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not be '.' or '..'"))
	}
	if strings.ContainsRune(name, 0) {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not contain null bytes"))
	}
	return nil
}

// listDirectory reads the immediate children of dir and returns them as
// FolderEntry slices. Folders get a trailing "/".
func listDirectory(dir string) ([]*folderv1.FolderEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	result := make([]*folderv1.FolderEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		result = append(result, &folderv1.FolderEntry{Path: name})
	}
	return result, nil
}
