package tasks

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

	// Validate name
	name := req.GetName()
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not be empty"))
	}
	if strings.ContainsAny(name, "/\\") {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name must not contain path separators"))
	}

	// Validate tasks
	domainTasks := protoToMainTasks(req.GetTasks())
	for i, t := range domainTasks {
		if t.DueDate != "" && t.Recurrence != "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task %d: cannot set both due_date and recurrence", i))
		}
		if t.Recurrence != "" {
			if err := ValidateRRule(t.Recurrence); err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			next, err := ComputeNextDueDate(t.Recurrence, time.Now())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			domainTasks[i].DueDate = next.Format("2006-01-02")
		}
	}

	// Create intermediate directories
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create directory: %w", err))
	}

	// Build file path
	filename := "tasks_" + name + ".md"
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
		TaskList: buildTaskList(relPath, name, domainTasks, nowMillis()),
	}, nil
}
