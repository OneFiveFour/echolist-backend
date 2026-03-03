package notes

import (
	"context"
	"errors"
	"fmt"
	"os"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) DeleteNote(
	ctx context.Context,
	req *pb.DeleteNoteRequest,
) (*pb.DeleteNoteResponse, error) {

	absPath, err := common.ValidatePath(s.dataDir, req.GetFilePath())
	if err != nil {
		return nil, err
	}

	if err := common.ValidateFileType(absPath, common.NoteFileType); err != nil {
		return nil, err
	}

	unlock := s.locks.Lock(absPath)
	defer unlock()

	if err := os.Remove(absPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found: %w", err))
		}
		s.logger.Error("failed to delete note", "path", req.GetFilePath(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete note: %w", err))
	}

	return &pb.DeleteNoteResponse{}, nil
}
