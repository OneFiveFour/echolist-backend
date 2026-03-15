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

func (s *NotesServer) UpdateNote(
	ctx context.Context,
	req *pb.UpdateNoteRequest,
) (*pb.UpdateNoteResponse, error) {

	// TODO: Task 7 will implement ID-based lookup. Minimal stub to compile.
	absPath, err := common.ValidatePath(s.dataDir, req.GetId())
	if err != nil {
		return nil, err
	}

	if err := common.ValidateFileType(absPath, common.NoteFileType); err != nil {
		return nil, err
	}

	if err := common.ValidateContentLength(req.GetContent(), common.MaxNoteContentBytes, "content"); err != nil {
		return nil, err
	}

	unlock := s.locks.Lock(absPath)
	defer unlock()

	if _, err := os.Stat(absPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		s.logger.Error("failed to stat note", "path", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note: %w", err))
	}

	err = common.File(absPath, []byte(req.Content))
	if err != nil {
		s.logger.Error("failed to update note", "path", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update note: %w", err))
	}

	info, err := os.Stat(absPath)
	if err != nil {
		s.logger.Error("failed to stat note after update", "path", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note after update: %w", err))
	}

	title, err := ExtractNoteTitle(info.Name())
	if err != nil {
		s.logger.Error("failed to extract note title", "path", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	note := &pb.Note{
		FilePath:  req.Id,
		Title:     title,
		Content:   req.Content,
		UpdatedAt: info.ModTime().UnixMilli(),
	}

	return &pb.UpdateNoteResponse{Note: note}, nil
}
