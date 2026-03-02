package notes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
	if err := pathutil.ValidateName(title); err != nil {
		return nil, err
	}

	// Only allow creating notes in existing directories (depth limit = 1).
	// Reject requests that would auto-create intermediate directories.
	if info, err := os.Stat(dirPath); err != nil || !info.IsDir() {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("parent directory does not exist"))
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

	info, err := os.Stat(absoluteFilePath)
	if err != nil {
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
