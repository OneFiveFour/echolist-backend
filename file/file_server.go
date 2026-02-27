package file

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/proto/gen/file/v1/filev1connect"
)

type FileServer struct {
	filev1connect.UnimplementedFileServiceHandler
	dataDir string
}

func NewFileServer(dataDir string) *FileServer {
	return &FileServer{dataDir: dataDir}
}

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
