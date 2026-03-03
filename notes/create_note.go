package notes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) CreateNote(
	ctx context.Context,
	req *pb.CreateNoteRequest,
) (*pb.CreateNoteResponse, error) {

	// Validate path
	dirPath, err := common.ValidateParentDir(s.dataDir, req.GetParentDir())
	if err != nil {
		return nil, err
	}

	title := req.GetTitle()
	if err := common.ValidateName(title); err != nil {
		return nil, err
	}

	if err := common.ValidateContentLength(req.GetContent(), common.MaxNoteContentBytes, "content"); err != nil {
		return nil, err
	}

	if err := common.RequireDir(dirPath, "parent directory"); err != nil {
		return nil, err
	}

	filename := common.NoteFileType.Prefix + title + common.NoteFileType.Suffix
	absoluteFilePath := filepath.Join(dirPath, filename)
	relativeFilePath, _ := filepath.Rel(s.dataDir, absoluteFilePath)

	unlock := s.locks.Lock(absoluteFilePath)
	defer unlock()

	// Use exclusive create to avoid TOCTOU race between existence check and write.
	err = common.CreateExclusive(absoluteFilePath, []byte(req.Content))
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("note already exists"))
		}
		s.logger.Error("failed to write note", "path", relativeFilePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write note: %w", err))
	}

	info, err := os.Stat(absoluteFilePath)
	if err != nil {
		s.logger.Error("failed to stat note after create", "path", relativeFilePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note after create: %w", err))
	}

	note := &pb.Note{
		FilePath:  relativeFilePath,
		Title:     req.Title,
		Content:   req.Content,
		UpdatedAt: info.ModTime().UnixMilli(),
	}

	return &pb.CreateNoteResponse{Note: note}, nil

}
