package folder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	folderv1 "notes-backend/proto/gen/folder/v1"
	"notes-backend/proto/gen/folder/v1/folderv1connect"
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

// isSubPath checks that resolved is a child of base (prevents path traversal).
func isSubPath(base, resolved string) bool {
	rel, err := filepath.Rel(base, resolved)
	if err != nil {
		return false
	}
	// Must not start with ".." and must not be "."
	return !strings.HasPrefix(rel, "..") && rel != "."
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
// DirectoryEntry slices. Folders get a trailing "/".
func listDirectory(dir string) ([]*folderv1.DirectoryEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	result := make([]*folderv1.DirectoryEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		result = append(result, &folderv1.DirectoryEntry{Path: name})
	}
	return result, nil
}
