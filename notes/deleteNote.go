package notes

import (
	"context"
	"fmt"
	"os"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) DeleteNote(
	ctx context.Context,
	req *pb.DeleteNoteRequest,
) (*pb.DeleteNoteResponse, error) {

	absPath, err := pathutil.ValidatePath(s.dataDir, req.GetFilePath())
	if err != nil {
		return nil, err
	}

	if err := pathutil.ValidateFileType(absPath, pathutil.FileType{
		Prefix: "note_", Suffix: ".md", Label: "note",
	}); err != nil {
		return nil, err
	}

	if err := os.Remove(absPath); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete note: %w", err))
	}

	return &pb.DeleteNoteResponse{}, nil
}
