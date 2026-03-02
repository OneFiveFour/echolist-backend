package file

import (
	"echolist-backend/pathlock"
	"echolist-backend/proto/gen/file/v1/filev1connect"
)

type FileServer struct {
	filev1connect.UnimplementedFileServiceHandler
	dataDir string
	locks   pathlock.Locker
}

func NewFileServer(dataDir string) *FileServer {
	return &FileServer{dataDir: dataDir}
}

