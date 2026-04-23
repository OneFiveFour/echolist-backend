package file

import (
	"log/slog"

	"echolist-backend/common"
	"echolist-backend/database"
	"echolist-backend/proto/gen/file/v1/filev1connect"
)

type FileServer struct {
	filev1connect.UnimplementedFileServiceHandler
	dataDir string
	db      *database.Database
	locks   common.Locker
	logger  *slog.Logger
}

func NewFileServer(dataDir string, db *database.Database, logger *slog.Logger) *FileServer {
	return &FileServer{dataDir: dataDir, db: db, logger: logger.With("service", "file")}
}

