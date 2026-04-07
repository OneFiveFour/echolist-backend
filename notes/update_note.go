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

func (s *NotesServer) UpdateNote(
	ctx context.Context,
	req *pb.UpdateNoteRequest,
) (*pb.UpdateNoteResponse, error) {
	if err := validateUuidV4(req.GetId()); err != nil {
		return nil, err
	}

	regPath := registryPath(s.dataDir)
	unlockReg := s.locks.Lock(regPath)
	defer unlockReg()
	entry, found, err := registryLookup(regPath, req.GetId())
	if err != nil {
		s.logger.Error("failed to read registry", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read registry: %w", err))
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
	}
	filePath := entry.FilePath

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

	title := req.GetTitle()
	if err := common.ValidateName(title); err != nil {
		return nil, err
	}

	// Titles are encoded into filenames (note_<title>.md), so changing the title
	// means changing the on-disk path as well as the returned metadata.
	parentDir := filepath.Dir(filePath)
	if parentDir == "." {
		parentDir = ""
	}
	newFileName := common.NoteFileType.Prefix + title + common.NoteFileType.Suffix
	newFilePath := filepath.Join(parentDir, newFileName)
	newAbsPath := filepath.Join(filepath.Dir(absPath), newFileName)

	unlockFiles := s.locks.LockMany(absPath, newAbsPath)
	defer unlockFiles()

	if _, err := os.Stat(absPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
		}
		s.logger.Error("failed to stat note", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note: %w", err))
	}

	currentAbsPath := absPath
	currentFilePath := filePath
	renamed := false

	if newAbsPath != absPath {
		// We must reject collisions up front because another note may already use
		// the target title in the same directory.
		if _, err := os.Stat(newAbsPath); err == nil {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("note already exists"))
		} else if !errors.Is(err, os.ErrNotExist) {
			s.logger.Error("failed to stat target note", "path", newFilePath, "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat target note: %w", err))
		}

		if err := os.Rename(absPath, newAbsPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("note not found"))
			}
			s.logger.Error("failed to rename note", "from", filePath, "to", newFilePath, "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to rename note: %w", err))
		}

		// The ID stays stable across renames, so the registry must move to the new
		// relative file path before later lookups by ID can succeed.
		if err := registryAdd(regPath, req.GetId(), registryEntry{FilePath: newFilePath}); err != nil {
			if rollbackErr := os.Rename(newAbsPath, absPath); rollbackErr != nil {
				s.logger.Error("failed to roll back note rename", "from", newFilePath, "to", filePath, "error", rollbackErr)
			}
			s.logger.Error("failed to update registry entry", "id", req.GetId(), "path", newFilePath, "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to persist note id: %w", err))
		}

		currentAbsPath = newAbsPath
		currentFilePath = newFilePath
		renamed = true
	}

	err = common.File(currentAbsPath, []byte(req.Content))
	if err != nil {
		if renamed {
			// If the content write fails after a rename, roll the visible path back
			// so the registry and filesystem keep pointing to the same note file.
			if rollbackErr := registryAdd(regPath, req.GetId(), registryEntry{FilePath: filePath}); rollbackErr != nil {
				s.logger.Error("failed to roll back note registry entry", "id", req.GetId(), "path", filePath, "error", rollbackErr)
			}
			if rollbackErr := os.Rename(currentAbsPath, absPath); rollbackErr != nil {
				s.logger.Error("failed to roll back note rename", "from", currentFilePath, "to", filePath, "error", rollbackErr)
			}
		}
		s.logger.Error("failed to update note", "path", currentFilePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update note: %w", err))
	}

	info, err := os.Stat(currentAbsPath)
	if err != nil {
		s.logger.Error("failed to stat note after update", "path", currentFilePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat note after update: %w", err))
	}

	note := &pb.Note{
		Id:        req.GetId(),
		FilePath:  currentFilePath,
		Title:     title,
		Content:   req.Content,
		UpdatedAt: info.ModTime().UnixMilli(),
	}

	return &pb.UpdateNoteResponse{Note: note}, nil
}
