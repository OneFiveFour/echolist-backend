package notes

import (
	"log/slog"

	"echolist-backend/common"
	notesv1connect "echolist-backend/proto/gen/notes/v1/notesv1connect"
)

type NotesServer struct {
	notesv1connect.UnimplementedNoteServiceHandler
	dataDir string
	locks   common.Locker
	logger  *slog.Logger
}

func NewNotesServer(dataDir string, logger *slog.Logger) *NotesServer {
	return &NotesServer{dataDir: dataDir, logger: logger.With("service", "notes")}
}
