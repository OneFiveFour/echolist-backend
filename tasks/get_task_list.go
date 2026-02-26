package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) GetTaskList(
	ctx context.Context,
	req *pb.GetTaskListRequest,
) (*pb.GetTaskListResponse, error) {
	absPath, err := pathutil.ValidatePath(s.dataDir, req.GetFilePath())
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read task file: %w", err))
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read task file: %w", err))
	}

	domainTasks, err := ParseTaskFile(data)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to parse task file: %w", err))
	}

	name := strings.TrimPrefix(strings.TrimSuffix(filepath.Base(absPath), ".md"), "tasks_")

	return &pb.GetTaskListResponse{
		FilePath:  req.GetFilePath(),
		Name:      name,
		Tasks:     mainTasksToProto(domainTasks),
		UpdatedAt: info.ModTime().UnixMilli(),
	}, nil
}
