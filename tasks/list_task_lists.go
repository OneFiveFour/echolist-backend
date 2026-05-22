package tasks

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) ListTaskLists(
	ctx context.Context,
	req *pb.ListTaskListsRequest,
) (*pb.ListTaskListsResponse, error) {
	parentDir := req.GetParentDir()

	dirPath, err := common.ValidateParentDir(s.dataDir, parentDir)
	if err != nil {
		return nil, err
	}

	err = common.RequireDir(dirPath)
	if err != nil {
		return nil, err
	}

	tlRows, tasksByList, err := s.db.ListTaskLists(parentDir)
	if err != nil {
		s.logger.Error("failed to list task lists", "parentDir", parentDir, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list task lists: %w", err))
	}

	taskLists := make([]*pb.TaskList, len(tlRows))
	for i, taskListRow := range tlRows {
		domainTasks := taskRowsToMainTasks(tasksByList[taskListRow.Id])
		taskLists[i] = buildTaskList(taskListRow.Id, taskListRow.ParentDir, taskListRow.Title, domainTasks, taskListRow.UpdatedAt, taskListRow.IsAutoDelete)
	}

	return &pb.ListTaskListsResponse{TaskLists: taskLists}, nil
}
