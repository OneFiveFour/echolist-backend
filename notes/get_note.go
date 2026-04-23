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

	// Validate the id field before any filesystem operations (Req 9.1, 9.2)
	if err := common.ValidateUuidV4(req.GetId()); err != nil {
		return nil, err
	}

	// Resolve id to a file path via the registry (Req 4.1, 4.2)
	regPath := registryPath(s.dataDir)
	entry, found, err := registryLookup(regPath, req.GetId())
	if err != nil {
		s.logger.Error("failed to read registry", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read registry: %w", err))
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
	}
	filePath := entry.FilePath

	// Validate the resolved path doesn't escape the data directory
	absPath, err := common.ValidatePath(s.dataDir, filePath)
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
		s.logger.Error("failed to stat note", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note: %w", err))
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		s.logger.Error("failed to read note", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read note: %w", err))
	}

	title, err := ExtractNoteTitle(info.Name())
	if err != nil {
		s.logger.Error("failed to extract note title", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	note := &pb.Note{
		Id:        req.GetId(),
		Title:     title,
		Content:   string(content),
		UpdatedAt: info.ModTime().UnixMilli(),
	}

	return &pb.GetNoteResponse{Note: note}, nil
}
