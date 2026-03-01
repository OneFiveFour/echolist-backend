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
	dirPath, err := pathutil.ValidateParentDir(s.dataDir, req.GetParentDir())
	if err != nil {
		return nil, err
	}

	title := req.GetTitle()
	if title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title must not be empty"))
	}
	if strings.ContainsAny(title, "/\\") {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title must not contain path separators"))
	}
	if strings.ContainsRune(title, 0) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title must not contain null bytes"))
	}

	err = os.MkdirAll(dirPath, 0755)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create directory: %w", err))
	}

	filename := "note_" + title + ".md"
	absoluteFilePath := filepath.Join(dirPath, filename)
	relativeFilePath, _ := filepath.Rel(s.dataDir, absoluteFilePath)

	// Check for existing file
	if _, err := os.Stat(absoluteFilePath); err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("note already exists"))
	}

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
