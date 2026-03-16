package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) GetTaskList(
	ctx context.Context,
	req *pb.GetTaskListRequest,
) (*pb.GetTaskListResponse, error) {
	absPath, err := common.ValidatePath(s.dataDir, req.GetId())
	if err != nil {
		return nil, err
	}

	if err := common.ValidateFileType(absPath, common.TaskListFileType); err != nil {
		return nil, err
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		s.logger.Error("failed to read task file", "path", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read task file: %w", err))
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		s.logger.Error("failed to read task file", "path", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to read task file: %w", err))
	}

	domainTasks, err := ParseTaskFile(data)
	if err != nil {
		s.logger.Error("failed to parse task file", "path", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to parse task file: %w", err))
	}

	title, err := ExtractTaskListTitle(filepath.Base(absPath))
	if err != nil {
		s.logger.Error("invalid task list filename", "path", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("invalid task list filename: %w", err))
	}

	return &pb.GetTaskListResponse{
		TaskList: buildTaskList("", req.GetId(), title, domainTasks, info.ModTime().UnixMilli()),
	}, nil
}
