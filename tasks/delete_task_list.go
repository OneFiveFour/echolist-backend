package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"

	"connectrpc.com/connect"

	"echolist-backend/pathutil"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) DeleteTaskList(
	ctx context.Context,
	req *pb.DeleteTaskListRequest,
) (*pb.DeleteTaskListResponse, error) {
	absPath, err := pathutil.ValidatePath(s.dataDir, req.GetFilePath())
	if err != nil {
		return nil, err
	}

	if err := pathutil.ValidateFileType(absPath, pathutil.TaskListFileType); err != nil {
		return nil, err
	}

	unlock := s.locks.Lock(absPath)
	defer unlock()

	if err := os.Remove(absPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found: %w", err))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete task file: %w", err))
	}

	return &pb.DeleteTaskListResponse{}, nil
}
