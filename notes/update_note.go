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

	// Validate the id field before any filesystem operations (Req 9.1, 9.2)
	if err := validateUuidV4(req.GetId()); err != nil {
		return nil, err
	}

	// Resolve id to a file path via the registry (Req 5.1, 5.2)
	regPath := registryPath(s.dataDir)
	filePath, found, err := registryLookup(regPath, req.GetId())
	if err != nil {
		s.logger.Error("failed to read registry", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read registry: %w", err))
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
	}

	// Validate the resolved path doesn't escape the data directory
	absPath, err := common.ValidatePath(s.dataDir, filePath)
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
		s.logger.Error("failed to stat note", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note: %w", err))
	}

	err = common.File(absPath, []byte(req.Content))
	if err != nil {
		s.logger.Error("failed to update note", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update note: %w", err))
	}

	info, err := os.Stat(absPath)
	if err != nil {
		s.logger.Error("failed to stat note after update", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note after update: %w", err))
	}

	title, err := ExtractNoteTitle(info.Name())
	if err != nil {
		s.logger.Error("failed to extract note title", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	note := &pb.Note{
		Id:        req.GetId(),
		FilePath:  filePath,
		Title:     title,
		Content:   req.Content,
		UpdatedAt: info.ModTime().UnixMilli(),
	}

	return &pb.UpdateNoteResponse{Note: note}, nil
}
