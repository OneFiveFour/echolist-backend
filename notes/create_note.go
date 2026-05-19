package notes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"echolist-backend/common"
	"echolist-backend/database"
	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) CreateNote(
	ctx context.Context,
	req *pb.CreateNoteRequest,
) (*pb.CreateNoteResponse, error) {

	// Validate parent directory path
	parentDir := req.GetParentDir()
	dirPath, err := common.ValidateParentDir(s.dataDir, parentDir)
	if err != nil {
		return nil, err
	}

	// Validate title
	title := req.GetTitle()
	err = common.ValidateName(title)
	if err != nil {
		return nil, err
	}

	// Validate content length
	content := req.GetContent()
	err = common.ValidateContentLength(content, common.MaxNoteContentBytes, "content")
	if err != nil {
		return nil, err
	}

	// Validate parent directory exists
	err = common.RequireDir(dirPath, "parent directory")
	if err != nil {
		return nil, err
	}

	// Generate Note_ID
	id := uuid.NewString()

	// Compute file path via NotePath helper
	notePath := NotePath(parentDir, title, id)
	absPath := filepath.Join(s.dataDir, notePath)

	// Lock the file path for concurrent safety
	unlockFile := s.locks.Lock(absPath)
	defer unlockFile()

	// Create file on disk first
	err = common.CreateExclusive(absPath, []byte(content))
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("note already exists"))
		}
		s.logger.Error("failed to write note", "path", notePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write note: %w", err))
	}

	// Get updated_at from file stat
	info, err := os.Stat(absPath)
	if err != nil {
		s.logger.Error("failed to stat note after create", "path", notePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note after create: %w", err))
	}
	updatedAt := info.ModTime().UnixMilli()

	// Compute preview: first 100 runes of content
	preview := computePreview(content)

	// Insert DB row
	err = s.db.InsertNote(database.InsertNoteParams{
		Id:        id,
		Title:     title,
		ParentDir: parentDir,
		Preview:   preview,
		CreatedAt: updatedAt,
		UpdatedAt: updatedAt,
	})
	if err != nil {
		// Rollback: delete the file we just created
		if removeErr := os.Remove(absPath); removeErr != nil {
			s.logger.Error("failed to rollback note file after DB insert failure", "path", notePath, "error", removeErr)
		}
		s.logger.Error("failed to insert note into database", "id", id, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to persist note metadata: %w", err))
	}

	note := &pb.Note{
		Id:        id,
		Title:     title,
		Content:   content,
		UpdatedAt: updatedAt,
		ParentDir: parentDir,
	}

	return &pb.CreateNoteResponse{Note: note}, nil
}

// computePreview returns the first 100 runes of content, or the full content
// if it is shorter.
func computePreview(content string) string {
	runes := []rune(content)
	if len(runes) > 100 {
		return string(runes[:100])
	}
	return content
}
