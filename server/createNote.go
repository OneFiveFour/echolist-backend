package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"

	"echolist-backend/atomicwrite"
	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) CreateNote(
	ctx context.Context,
	req *pb.CreateNoteRequest,
) (*pb.CreateNoteResponse, error) {

	// Validate path
	dirPath := filepath.Clean(filepath.Join(s.dataDir, req.GetParentDir()))
	if dirPath != s.dataDir && !pathutil.IsSubPath(s.dataDir, dirPath) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("path escapes data directory"))
	}

	title := req.GetTitle()
	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title must not be empty"))
	}
	if strings.ContainsAny(title, "/\\") {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title must not contain path separators"))
	}

	destination := filepath.Join(s.dataDir, req.ParentDir)

	err := os.MkdirAll(destination, 0755)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create directory: %w", err))
	}

	relativeFilePath := filepath.Join(req.ParentDir, "note_"+req.Title+".md")
	absoluteFilePath := filepath.Join(s.dataDir, relativeFilePath)

	err = atomicwrite.File(absoluteFilePath, []byte(req.Content))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write note: %w", err))
	}

	note := &pb.Note{
		FilePath:  relativeFilePath,
		Title:     req.Title,
		Content:   req.Content,
		UpdatedAt: time.Now().UnixMilli(),
	}

	return &pb.CreateNoteResponse{Note: note}, nil

}
