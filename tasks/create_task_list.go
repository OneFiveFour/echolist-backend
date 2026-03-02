package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"

	"echolist-backend/atomicwrite"
	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) CreateTaskList(
	ctx context.Context,
	req *pb.CreateTaskListRequest,
) (*pb.CreateTaskListResponse, error) {
	// Validate path
	dirPath, err := pathutil.ValidateParentDir(s.dataDir, req.GetParentDir())
	if err != nil {
		return nil, err
	}

	// Validate title
	title := req.GetTitle()
	if err := pathutil.ValidateName(title); err != nil {
		return nil, err
	}

	// Validate tasks
	domainTasks := protoToMainTasks(req.GetTasks())
	if err := validateTasks(domainTasks); err != nil {
		return nil, err
	}
	for i, t := range domainTasks {
		if t.Recurrence != "" {
			next, err := ComputeNextDueDate(t.Recurrence, time.Now())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			domainTasks[i].DueDate = next.Format("2006-01-02")
		}
	}

	// Only allow creating task lists in existing directories (depth limit = 1).
	// Reject requests that would auto-create intermediate directories.
	if err := pathutil.RequireDir(dirPath, "parent directory"); err != nil {
		return nil, err
	}

	// Build file path
	filename := pathutil.TaskListFileType.Prefix + title + pathutil.TaskListFileType.Suffix
	absPath := filepath.Join(dirPath, filename)

	// Check for existing file
	if _, err := os.Stat(absPath); err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("task list already exists"))
	}

	// Write file atomically
	data := PrintTaskFile(domainTasks)
	if err := atomicwrite.File(absPath, data); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write task file: %w", err))
	}

	relPath := filepath.Join(req.GetParentDir(), filename)
	return &pb.CreateTaskListResponse{
		TaskList: buildTaskList(relPath, title, domainTasks, nowMillis()),
	}, nil
}
