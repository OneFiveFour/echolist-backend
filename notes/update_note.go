package notes

import (
	"context"
	"errors"
	"fmt"
	"os"

	"connectrpc.com/connect"

	"echolist-backend/atomicwrite"
	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) UpdateNote(
	ctx context.Context,
	req *pb.UpdateNoteRequest,
) (*pb.UpdateNoteResponse, error) {

	absPath, err := pathutil.ValidatePath(s.dataDir, req.GetFilePath())
	if err != nil {
		return nil, err
	}

	if err := pathutil.ValidateFileType(absPath, pathutil.NoteFileType); err != nil {
		return nil, err
	}

	if err := pathutil.ValidateContentLength(req.GetContent(), pathutil.MaxNoteContentBytes, "content"); err != nil {
		return nil, err
	}

	unlock := s.locks.Lock(absPath)
	defer unlock()

	if _, err := os.Stat(absPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note: %w", err))
	}

	err = atomicwrite.File(absPath, []byte(req.Content))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update note: %w", err))
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note after update: %w", err))
	}

	title, err := ExtractNoteTitle(info.Name())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	note := &pb.Note{
		FilePath:  req.FilePath,
		Title:     title,
		Content:   req.Content,
		UpdatedAt: info.ModTime().UnixMilli(),
	}

	return &pb.UpdateNoteResponse{Note: note}, nil
}
