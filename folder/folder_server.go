package folder

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"

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
