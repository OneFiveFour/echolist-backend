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

func (s *TaskServer) CreateTaskList(
	ctx context.Context,
	req *pb.CreateTaskListRequest,
) (*pb.CreateTaskListResponse, error) {
	// Validate path
	dirPath, err := common.ValidateParentDir(s.dataDir, req.GetParentDir())
	if err != nil {
		return nil, err
	}

	title := req.GetTitle()
	if err := common.ValidateName(title); err != nil {
		return nil, err
	}

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

	if err := common.RequireDir(dirPath, "parent directory"); err != nil {
		return nil, err
	}

	filename := common.TaskListFileType.Prefix + title + common.TaskListFileType.Suffix
	absPath := filepath.Join(dirPath, filename)

	unlock := s.locks.Lock(absPath)
	defer unlock()

	// Use exclusive create to avoid TOCTOU race between existence check and write.
	data := PrintTaskFile(domainTasks)
	if err := common.CreateExclusive(absPath, data); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("task list already exists"))
		}
		s.logger.Error("failed to write task file", "path", absPath, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to write task file: %w", err))
	}

	relPath := filepath.Join(req.GetParentDir(), filename)
	return &pb.CreateTaskListResponse{
		TaskList: buildTaskList(relPath, title, domainTasks, nowMillis()),
	}, nil
}
