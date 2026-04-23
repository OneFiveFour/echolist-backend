package notes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"echolist-backend/common"
	"echolist-backend/database"
	pb "echolist-backend/proto/gen/notes/v1"
)

func (s *NotesServer) UpdateNote(
	ctx context.Context,
	req *pb.UpdateNoteRequest,
) (*pb.UpdateNoteResponse, error) {

	// Validate ID
	if err := common.ValidateUuidV4(req.GetId()); err != nil {
		return nil, err
	}

	// Validate title
	title := req.GetTitle()
	if err := common.ValidateName(title); err != nil {
		return nil, err
	}

	// Validate content
	if err := common.ValidateContentLength(req.GetContent(), common.MaxNoteContentBytes, "content"); err != nil {
		return nil, err
	}

	// Query DB for current metadata
	noteRow, err := s.db.GetNote(req.GetId())
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		s.logger.Error("failed to query note", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to query note: %w", err))
	}

	// Compute old file path from current metadata
	oldNotePath := database.NotePath(noteRow.ParentDir, noteRow.Title, noteRow.Id)
	oldAbsPath := filepath.Join(s.dataDir, oldNotePath)

	// Compute new file path (title may have changed)
	newNotePath := database.NotePath(noteRow.ParentDir, title, noteRow.Id)
	newAbsPath := filepath.Join(s.dataDir, newNotePath)

	// Lock both file paths for concurrent safety
	unlockFiles := s.locks.LockMany(oldAbsPath, newAbsPath)
	defer unlockFiles()

	// Check that the old file exists on disk
	if _, err := os.Stat(oldAbsPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		s.logger.Error("failed to stat note", "path", oldNotePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note: %w", err))
	}

	// If title changed, rename file on disk
	renamed := false
	if newAbsPath != oldAbsPath {
		if err := os.Rename(oldAbsPath, newAbsPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
			}
			s.logger.Error("failed to rename note", "from", oldNotePath, "to", newNotePath, "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to rename note: %w", err))
		}
		renamed = true
	}

	// Write new content to file
	currentAbsPath := newAbsPath
	currentNotePath := newNotePath
	err = common.File(currentAbsPath, []byte(req.GetContent()))
	if err != nil {
		if renamed {
			// Rollback: rename file back
			if rollbackErr := os.Rename(newAbsPath, oldAbsPath); rollbackErr != nil {
				s.logger.Error("failed to rollback note rename", "from", newNotePath, "to", oldNotePath, "error", rollbackErr)
			}
		}
		s.logger.Error("failed to write note content", "path", currentNotePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update note: %w", err))
	}

	// Get updated_at from file stat after write
	info, err := os.Stat(currentAbsPath)
	if err != nil {
		s.logger.Error("failed to stat note after update", "path", currentNotePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note after update: %w", err))
	}
	updatedAt := info.ModTime().UnixMilli()

	// Compute new preview
	preview := computePreview(req.GetContent())

	// Update DB row
	err = s.db.UpdateNote(req.GetId(), title, preview, updatedAt)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		// Rollback: if we renamed, rename back
		if renamed {
			if rollbackErr := os.Rename(newAbsPath, oldAbsPath); rollbackErr != nil {
				s.logger.Error("failed to rollback note rename after DB failure", "from", newNotePath, "to", oldNotePath, "error", rollbackErr)
			}
		}
		s.logger.Error("failed to update note in database", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update note metadata: %w", err))
	}

	note := &pb.Note{
		Id:        req.GetId(),
		Title:     title,
		Content:   req.GetContent(),
		UpdatedAt: updatedAt,
	}

	return &pb.UpdateNoteResponse{Note: note}, nil
}
