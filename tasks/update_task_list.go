package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) UpdateTaskList(
	ctx context.Context,
	req *pb.UpdateTaskListRequest,
) (*pb.UpdateTaskListResponse, error) {
	if err := validateUuidV4(req.GetId()); err != nil {
		return nil, err
	}

	regPath := registryPath(s.dataDir)
	unlockReg := s.locks.Lock(regPath)
	defer unlockReg()

	filePath, found, err := registryLookup(regPath, req.GetId())
	if err != nil {
		s.logger.Error("failed to read registry", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read registry: %w", err))
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
	}

	absPath, err := common.ValidatePath(s.dataDir, filePath)
	if err != nil {
		return nil, err
	}

	if err := common.ValidateFileType(absPath, common.TaskListFileType); err != nil {
		return nil, err
	}

	title := req.GetTitle()
	if err := common.ValidateName(title); err != nil {
		return nil, err
	}

	// Titles are encoded into filenames (tasks_<title>.md), so changing the
	// title means renaming the backing file and updating the registry path.
	parentDir := filepath.Dir(filePath)
	if parentDir == "." {
		parentDir = ""
	}
	newFileName := common.TaskListFileType.Prefix + title + common.TaskListFileType.Suffix
	newFilePath := filepath.Join(parentDir, newFileName)
	newAbsPath := filepath.Join(filepath.Dir(absPath), newFileName)

	unlockFiles := s.locks.LockMany(absPath, newAbsPath)
	defer unlockFiles()

	existingData, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		s.logger.Error("failed to read task file", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read task file: %w", err))
	}

	existingTasks, err := ParseTaskFile(existingData)
	if err != nil {
		s.logger.Error("failed to parse task file", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to parse task file: %w", err))
	}

	domainTasks := protoToMainTasks(req.GetTasks())
	if err := validateTasks(domainTasks); err != nil {
		return nil, err
	}

	type taskKey struct{ desc, recurrence string }
	existingByKey := make(map[taskKey]MainTask, len(existingTasks))
	for _, et := range existingTasks {
		if et.Recurrence != "" {
			existingByKey[taskKey{et.Description, et.Recurrence}] = et
		}
	}

	for i, t := range domainTasks {
		if t.Recurrence == "" || !t.Done {
			continue
		}
		var prevDue string
		if et, ok := existingByKey[taskKey{t.Description, t.Recurrence}]; ok {
			prevDue = et.DueDate
		} else {
			prevDue = t.DueDate
		}

		after := time.Now()
		if prevDue != "" {
			if parsed, err := time.Parse("2006-01-02", prevDue); err == nil {
				after = parsed
			}
		}

		next, err := ComputeNextDueDate(t.Recurrence, after)
		if err != nil {
			s.logger.Error("failed to compute next due date", "path", filePath, "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to compute next due date: %w", err))
		}
		domainTasks[i].Done = false
		domainTasks[i].DueDate = next.Format("2006-01-02")
	}

	currentAbsPath := absPath
	currentFilePath := filePath
	renamed := false

	if newAbsPath != absPath {
		// A title rename maps directly to a new filename, so we must guard against
		// collisions with an existing task list in the same directory.
		if _, err := os.Stat(newAbsPath); err == nil {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("task list already exists"))
		} else if !errors.Is(err, os.ErrNotExist) {
			s.logger.Error("failed to stat target task file", "path", newFilePath, "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat target task file: %w", err))
		}

		if err := os.Rename(absPath, newAbsPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
			}
			s.logger.Error("failed to rename task file", "from", filePath, "to", newFilePath, "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to rename task file: %w", err))
		}

		// The task-list ID remains stable, so its registry entry must be updated
		// to the new relative file path after the rename.
		if err := registryAdd(regPath, req.GetId(), newFilePath); err != nil {
			if rollbackErr := os.Rename(newAbsPath, absPath); rollbackErr != nil {
				s.logger.Error("failed to roll back task list rename", "from", newFilePath, "to", filePath, "error", rollbackErr)
			}
			s.logger.Error("failed to update registry entry", "id", req.GetId(), "path", newFilePath, "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to persist task list id: %w", err))
		}

		currentAbsPath = newAbsPath
		currentFilePath = newFilePath
		renamed = true
	}

	data := PrintTaskFile(domainTasks)
	if err := common.File(currentAbsPath, data); err != nil {
		if renamed {
			// If persisting the updated tasks fails after a rename, roll the path
			// back so future ID lookups still match what is on disk.
			if rollbackErr := registryAdd(regPath, req.GetId(), filePath); rollbackErr != nil {
				s.logger.Error("failed to roll back task list registry entry", "id", req.GetId(), "path", filePath, "error", rollbackErr)
			}
			if rollbackErr := os.Rename(currentAbsPath, absPath); rollbackErr != nil {
				s.logger.Error("failed to roll back task list rename", "from", currentFilePath, "to", filePath, "error", rollbackErr)
			}
		}
		s.logger.Error("failed to write task file", "path", currentFilePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write task file: %w", err))
	}

	info, err := os.Stat(currentAbsPath)
	if err != nil {
		s.logger.Error("failed to stat task file after update", "path", currentFilePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat task file after update: %w", err))
	}

	return &pb.UpdateTaskListResponse{
		TaskList: buildTaskList(req.GetId(), currentFilePath, title, domainTasks, info.ModTime().UnixMilli()),
	}, nil
}
