package tasks

import (
	"context"
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

	if err := pathutil.ValidateFileType(absPath, pathutil.FileType{
		Prefix: "tasks_", Suffix: ".md", Label: "task list",
	}); err != nil {
		return nil, err
	}

	if err := os.Remove(absPath); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete task file: %w", err))
	}

	return &pb.DeleteTaskListResponse{}, nil
}
