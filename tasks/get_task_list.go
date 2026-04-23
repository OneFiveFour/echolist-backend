package tasks

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"echolist-backend/common"
	"echolist-backend/database"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) GetTaskList(
	ctx context.Context,
	req *pb.GetTaskListRequest,
) (*pb.GetTaskListResponse, error) {

	if err := common.ValidateUuidV4(req.GetId()); err != nil {
		return nil, err
	}

	tlRow, taskRows, err := s.db.GetTaskList(req.GetId())
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		s.logger.Error("failed to get task list", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get task list: %w", err))
	}

	domainTasks := taskRowsToMainTasks(taskRows)

	return &pb.GetTaskListResponse{
		TaskList: buildTaskList(tlRow.Id, tlRow.ParentDir, tlRow.Title, domainTasks, tlRow.UpdatedAt, tlRow.IsAutoDelete),
	}, nil
}
