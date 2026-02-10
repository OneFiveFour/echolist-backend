package server

import (
	pb "notes-backend/gen/notes"
)

type NotesServer struct {
	pb.UnimplementedNotesServiceServer
}
