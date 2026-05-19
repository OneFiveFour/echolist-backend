package tasks

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) DeleteTaskList(
	ctx context.Context,
	req *pb.DeleteTaskListRequest,
) (*pb.DeleteTaskListResponse, error) {

	// Validate ID
	id := req.GetId()
	err := common.ValidateUuidV4(id)
	if err != nil {
		return nil, err
	}

	found, err := s.db.DeleteTaskList(id)
	if err != nil {
		s.logger.Error("failed to delete task list", "id", id, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete task list: %w", err))
	}
	if !found {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
	}

	return &pb.DeleteTaskListResponse{}, nil
}
