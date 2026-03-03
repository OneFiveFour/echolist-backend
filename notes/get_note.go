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

func (s *NotesServer) GetNote(
	ctx context.Context,
	req *pb.GetNoteRequest,
) (*pb.GetNoteResponse, error) {

	absPath, err := common.ValidatePath(s.dataDir, req.GetFilePath())
	if err != nil {
		return nil, err
	}

	if err := common.ValidateFileType(absPath, common.NoteFileType); err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		s.logger.Error("failed to stat note", "path", req.GetFilePath(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note: %w", err))
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		s.logger.Error("failed to read note", "path", req.GetFilePath(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read note: %w", err))
	}

	title, err := ExtractNoteTitle(info.Name())
	if err != nil {
		s.logger.Error("failed to extract note title", "path", req.GetFilePath(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	note := &pb.Note{
		FilePath:  req.FilePath,
		Title:     title,
		Content:   string(content),
		UpdatedAt: info.ModTime().UnixMilli(),
	}

	return &pb.GetNoteResponse{Note: note}, nil
}
