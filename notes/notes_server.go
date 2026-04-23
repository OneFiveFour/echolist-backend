package notes

import (
	"log/slog"

	"echolist-backend/common"
	"echolist-backend/database"
	notesv1connect "echolist-backend/proto/gen/notes/v1/notesv1connect"
)

type NotesServer struct {
	notesv1connect.UnimplementedNoteServiceHandler
	dataDir string
	db      *database.Database
	locks   common.Locker
	logger  *slog.Logger
}

func NewNotesServer(dataDir string, db *database.Database, logger *slog.Logger) *NotesServer {
	return &NotesServer{dataDir: dataDir, db: db, logger: logger.With("service", "notes")}
}
