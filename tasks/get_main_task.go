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

func (s *TaskServer) GetMainTask(
	ctx context.Context,
	req *pb.GetMainTaskRequest,
) (*pb.GetMainTaskResponse, error) {

	// Validate ID
	id := req.GetId()
	err := common.ValidateUuidV4(id)
	if err != nil {
		return nil, err
	}

	mainRow, subRows, err := s.db.GetMainTask(id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("main task not found"))
		}
		s.logger.Error("failed to get main task", "id", id, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get main task: %w", err))
	}

	domainTask := singleTaskRowToMainTask(mainRow, subRows)

	return &pb.GetMainTaskResponse{
		MainTask: mainTaskToProto(domainTask),
	}, nil
}

// singleTaskRowToMainTask converts a main task row and its subtask rows into a domain MainTask.
func singleTaskRowToMainTask(mainRow database.TaskRow, subRows []database.TaskRow) MainTask {
	mainTask := MainTask{
		Id:          mainRow.Id,
		Description: mainRow.Description,
		IsDone:      mainRow.IsDone,
	}
	if mainRow.DueDate != nil {
		mainTask.DueDate = *mainRow.DueDate
	}
	if mainRow.Recurrence != nil {
		mainTask.Recurrence = *mainRow.Recurrence
	}
	for _, subRow := range subRows {
		mainTask.SubTasks = append(mainTask.SubTasks, SubTask{
			Id:          subRow.Id,
			Description: subRow.Description,
			IsDone:      subRow.IsDone,
		})
	}
	return mainTask
}

// mainTaskToProto converts a single domain MainTask to its proto representation.
func mainTaskToProto(task MainTask) *pb.MainTask {
	return &pb.MainTask{
		Id:          task.Id,
		Description: task.Description,
		IsDone:      task.IsDone,
		DueDate:     task.DueDate,
		Recurrence:  task.Recurrence,
		SubTasks:    subtasksToProto(task.SubTasks),
	}
}
