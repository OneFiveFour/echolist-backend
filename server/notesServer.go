package server

import (
	pb "notes-backend/gen/notes"
)

type NotesServer struct {
	pb.UnimplementedNotesServiceServer
	dataDir string
}

func NewNotesServer(dataDir string) *NotesServer {
	return &NotesServer{dataDir: dataDir}
}
