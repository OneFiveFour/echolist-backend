package file

import (
	"log/slog"

	"echolist-backend/pathlock"
	"echolist-backend/proto/gen/file/v1/filev1connect"
)

type FileServer struct {
	filev1connect.UnimplementedFileServiceHandler
	dataDir string
	locks   pathlock.Locker
	logger  *slog.Logger
}

func NewFileServer(dataDir string, logger *slog.Logger) *FileServer {
	return &FileServer{dataDir: dataDir, logger: logger.With("service", "file")}
}

