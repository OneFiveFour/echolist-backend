package tasks

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
)

// ListMainTaskPeek aggregates the main tasks of every task list directly in the
// requested parent directory into a single flat list. Subtasks are omitted.
// Task lists in subdirectories are not included. The returned tasks are grouped
// per task list and ordered within each list by stored position.
func (s *TaskServer) ListMainTaskPeek(
	ctx context.Context,
	req *pb.ListMainTaskPeekRequest,
) (*pb.ListMainTaskPeekResponse, error) {
	parentDir := req.GetParentDir()

	dirPath, err := common.ValidateParentDir(s.dataDir, parentDir)
	if err != nil {
		return nil, err
	}

	err = common.RequireDir(dirPath)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.ListMainTaskPeek(parentDir)
	if err != nil {
		s.logger.Error("failed to list main task peek", "parentDir", parentDir, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list main task peek: %w", err))
	}

	tasks := make([]*pb.MainTaskPeek, len(rows))
	for i, row := range rows {
		dueDate := ""
		if row.DueDate != nil {
			dueDate = *row.DueDate
		}
		tasks[i] = &pb.MainTaskPeek{
			MainTaskId:   row.MainTaskId,
			Description:  row.Description,
			IsDone:       row.IsDone,
			DueDate:      dueDate,
			TaskListId:   row.TaskListId,
			TaskListName: row.TaskListName,
		}
	}

	return &pb.ListMainTaskPeekResponse{Tasks: tasks}, nil
}
