package server

import (
	"context"
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

	fullPath := absPath

	err = atomicwrite.File(fullPath, []byte(req.Content))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update note: %w", err))
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note after update: %w", err))
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read note after update: %w", err))
	}

	title, err := ExtractNoteTitle(info.Name())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	note := &pb.Note{
		FilePath:  req.FilePath,
		Title:     title,
		Content:   string(content),
		UpdatedAt: info.ModTime().UnixMilli(),
	}

	return &pb.UpdateNoteResponse{Note: note}, nil
}
