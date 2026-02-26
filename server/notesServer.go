package server

import (
	notesv1connect "echolist-backend/proto/gen/notes/v1/notesv1connect"
)

type NotesServer struct {
	notesv1connect.UnimplementedNoteServiceHandler
	dataDir string
}

func NewNotesServer(dataDir string) *NotesServer {
	return &NotesServer{dataDir: dataDir}
}
