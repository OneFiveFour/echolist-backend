package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"

	"echolist-backend/atomicwrite"
	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) UpdateTaskList(
	ctx context.Context,
	req *pb.UpdateTaskListRequest,
) (*pb.UpdateTaskListResponse, error) {
	absPath, err := pathutil.ValidatePath(s.dataDir, req.GetFilePath())
	if err != nil {
		return nil, err
	}

	if err := pathutil.ValidateFileType(absPath, pathutil.TaskListFileType); err != nil {
		return nil, err
	}

	// Read existing file to compare recurring task state
	existingData, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read task file: %w", err))
	}

	existingTasks, err := ParseTaskFile(existingData)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to parse task file: %w", err))
	}

	// Validate and process incoming tasks
	domainTasks := protoToMainTasks(req.GetTasks())
	if err := validateTasks(domainTasks); err != nil {
		return nil, err
	}

	// Build a lookup of existing tasks keyed by (description, recurrence)
	// so recurring task matching is order-independent.
	type taskKey struct{ desc, recurrence string }
	existingByKey := make(map[taskKey]MainTask, len(existingTasks))
	for _, et := range existingTasks {
		if et.Recurrence != "" {
			existingByKey[taskKey{et.Description, et.Recurrence}] = et
		}
	}

	// Handle recurring tasks marked done: reset to open, advance due date
	for i, t := range domainTasks {
		if t.Recurrence == "" || !t.Done {
			continue
		}
		// Find matching existing task by identity, not position
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
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to compute next due date: %w", err))
		}
		domainTasks[i].Done = false
		domainTasks[i].DueDate = next.Format("2006-01-02")
	}

	// Write updated file atomically
	data := PrintTaskFile(domainTasks)
	if err := atomicwrite.File(absPath, data); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write task file: %w", err))
	}

	title, err := ExtractTaskListTitle(filepath.Base(absPath))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid task list filename: %w", err))
	}

	return &pb.UpdateTaskListResponse{
		TaskList: buildTaskList(req.GetFilePath(), title, domainTasks, nowMillis()),
	}, nil
}
