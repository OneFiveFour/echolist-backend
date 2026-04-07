package tasks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

	regEntry, found, err := registryLookup(regPath, req.GetId())
	if err != nil {
		s.logger.Error("failed to read registry", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read registry: %w", err))
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
	}

	filePath := regEntry.FilePath

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

	existingTasks, err := readAndParseTaskFile(absPath, filePath, s.logger)
	if err != nil {
		return nil, err
	}

	domainTasks := protoToMainTasks(req.GetTasks())
	if err := validateTasks(domainTasks); err != nil {
		return nil, err
	}

	if err := advanceRecurringTasks(domainTasks, existingTasks); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to compute next due date: %w", err))
	}

	isAutoDelete := req.GetIsAutoDelete()

	// Apply AutoDelete filtering after recurrence advancement
	if isAutoDelete {
		domainTasks = filterAutoDeleted(domainTasks)
	}

	rr, err := renameTaskFile(s.logger, renameParams{
		oldAbsPath:   absPath,
		newAbsPath:   newAbsPath,
		oldFilePath:  filePath,
		newFilePath:  newFilePath,
		regPath:      regPath,
		id:           req.GetId(),
		isAutoDelete: isAutoDelete,
	})
	if err != nil {
		return nil, err
	}

	if err := persistTaskFile(s.logger, persistParams{
		absPath:      rr.absPath,
		filePath:     rr.filePath,
		origAbsPath:  absPath,
		origFilePath: filePath,
		regPath:      regPath,
		id:           req.GetId(),
		isAutoDelete: isAutoDelete,
		renamed:      rr.renamed,
		tasks:        domainTasks,
	}); err != nil {
		return nil, err
	}

	// Always persist the (possibly updated) isAutoDelete flag in the registry.
	if !rr.renamed {
		if err := registryAdd(regPath, req.GetId(), registryEntry{FilePath: rr.filePath, IsAutoDelete: isAutoDelete}); err != nil {
			s.logger.Error("failed to update registry entry", "id", req.GetId(), "error", err)
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to persist task list id: %w", err))
		}
	}

	info, err := os.Stat(rr.absPath)
	if err != nil {
		s.logger.Error("failed to stat task file after update", "path", rr.filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat task file after update: %w", err))
	}

	return &pb.UpdateTaskListResponse{
		TaskList: buildTaskList(req.GetId(), rr.filePath, title, domainTasks, info.ModTime().UnixMilli(), isAutoDelete),
	}, nil
}

// readAndParseTaskFile reads and parses the markdown task file at absPath.
// filePath is the relative path used only for log/error messages.
func readAndParseTaskFile(absPath, filePath string, logger *slog.Logger) ([]MainTask, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		logger.Error("failed to read task file", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read task file: %w", err))
	}

	tasks, err := ParseTaskFile(data)
	if err != nil {
		logger.Error("failed to parse task file", "path", filePath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to parse task file: %w", err))
	}
	return tasks, nil
}

// advanceRecurringTasks resets done recurring tasks and computes their next due
// date based on the recurrence rule. existingTasks is used to look up the
// previous due date for matching recurring tasks. domainTasks is modified in place.
func advanceRecurringTasks(domainTasks, existingTasks []MainTask) error {
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
			return err
		}
		domainTasks[i].Done = false
		domainTasks[i].DueDate = next.Format("2006-01-02")
	}
	return nil
}

// renameParams holds the inputs for renameTaskFile.
type renameParams struct {
	oldAbsPath, newAbsPath   string
	oldFilePath, newFilePath string
	regPath, id              string
	isAutoDelete             bool
}

// renameResult holds the resolved paths after a potential rename.
type renameResult struct {
	absPath  string
	filePath string
	renamed  bool
}

// renameTaskFile renames the backing file when the title changes and updates
// the registry entry. If the paths are identical no rename is performed.
// On registry failure the file rename is rolled back.
func renameTaskFile(logger *slog.Logger, p renameParams) (renameResult, error) {
	if p.newAbsPath == p.oldAbsPath {
		return renameResult{absPath: p.oldAbsPath, filePath: p.oldFilePath, renamed: false}, nil
	}

	// Guard against collisions with an existing task list in the same directory.
	if _, err := os.Stat(p.newAbsPath); err == nil {
		return renameResult{}, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("task list already exists"))
	} else if !errors.Is(err, os.ErrNotExist) {
		logger.Error("failed to stat target task file", "path", p.newFilePath, "error", err)
		return renameResult{}, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to stat target task file: %w", err))
	}

	if err := os.Rename(p.oldAbsPath, p.newAbsPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return renameResult{}, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		logger.Error("failed to rename task file", "from", p.oldFilePath, "to", p.newFilePath, "error", err)
		return renameResult{}, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to rename task file: %w", err))
	}

	// The task-list ID remains stable, so its registry entry must be updated
	// to the new relative file path after the rename.
	if err := registryAdd(p.regPath, p.id, registryEntry{FilePath: p.newFilePath, IsAutoDelete: p.isAutoDelete}); err != nil {
		if rollbackErr := os.Rename(p.newAbsPath, p.oldAbsPath); rollbackErr != nil {
			logger.Error("failed to roll back task list rename", "from", p.newFilePath, "to", p.oldFilePath, "error", rollbackErr)
		}
		logger.Error("failed to update registry entry", "id", p.id, "path", p.newFilePath, "error", err)
		return renameResult{}, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to persist task list id: %w", err))
	}

	return renameResult{absPath: p.newAbsPath, filePath: p.newFilePath, renamed: true}, nil
}

// persistParams holds the inputs for persistTaskFile.
type persistParams struct {
	absPath, filePath           string
	origAbsPath, origFilePath   string
	regPath, id                 string
	isAutoDelete, renamed       bool
	tasks                       []MainTask
}

// persistTaskFile writes the task list to disk. If a rename preceded this call
// and the write fails, both the registry entry and the file are rolled back.
func persistTaskFile(logger *slog.Logger, p persistParams) error {
	data := PrintTaskFile(p.tasks)
	if err := common.File(p.absPath, data); err != nil {
		if p.renamed {
			if rollbackErr := registryAdd(p.regPath, p.id, registryEntry{FilePath: p.origFilePath, IsAutoDelete: p.isAutoDelete}); rollbackErr != nil {
				logger.Error("failed to roll back task list registry entry", "id", p.id, "path", p.origFilePath, "error", rollbackErr)
			}
			if rollbackErr := os.Rename(p.absPath, p.origAbsPath); rollbackErr != nil {
				logger.Error("failed to roll back task list rename", "from", p.filePath, "to", p.origFilePath, "error", rollbackErr)
			}
		}
		logger.Error("failed to write task file", "path", p.filePath, "error", err)
		return connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write task file: %w", err))
	}
	return nil
}

// filterAutoDeleted removes done tasks from the list when AutoDelete is enabled.
// It removes MainTasks where Done == true and Recurrence == "" (non-recurring),
// along with all their SubTasks. For surviving MainTasks, it removes SubTasks
// where Done == true. Returns a new slice; does not mutate the input.
func filterAutoDeleted(tasks []MainTask) []MainTask {
	var result []MainTask
	for _, mt := range tasks {
		if mt.Done && mt.Recurrence == "" {
			continue
		}
		filtered := MainTask{
			Description: mt.Description,
			Done:        mt.Done,
			DueDate:     mt.DueDate,
			Recurrence:  mt.Recurrence,
		}
		for _, st := range mt.SubTasks {
			if !st.Done {
				filtered.SubTasks = append(filtered.SubTasks, st)
			}
		}
		result = append(result, filtered)
	}
	return result
}
