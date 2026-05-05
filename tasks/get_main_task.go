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

	if err := common.ValidateUuidV4(req.GetId()); err != nil {
		return nil, err
	}

	mainRow, subRows, err := s.db.GetMainTask(req.GetId())
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("main task not found"))
		}
		s.logger.Error("failed to get main task", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get main task: %w", err))
	}

	domainTask := singleTaskRowToMainTask(mainRow, subRows)

	return &pb.GetMainTaskResponse{
		MainTask: mainTaskToProto(domainTask),
	}, nil
}

// singleTaskRowToMainTask converts a main task row and its subtask rows into a domain MainTask.
func singleTaskRowToMainTask(mainRow database.TaskRow, subRows []database.TaskRow) MainTask {
	mt := MainTask{
		Id:          mainRow.Id,
		Description: mainRow.Description,
		IsDone:      mainRow.IsDone,
	}
	if mainRow.DueDate != nil {
		mt.DueDate = *mainRow.DueDate
	}
	if mainRow.Recurrence != nil {
		mt.Recurrence = *mainRow.Recurrence
	}
	for _, r := range subRows {
		mt.SubTasks = append(mt.SubTasks, SubTask{
			Id:          r.Id,
			Description: r.Description,
			IsDone:      r.IsDone,
		})
	}
	return mt
}

// mainTaskToProto converts a single domain MainTask to its proto representation.
func mainTaskToProto(t MainTask) *pb.MainTask {
	return &pb.MainTask{
		Id:          t.Id,
		Description: t.Description,
		IsDone:      t.IsDone,
		DueDate:     t.DueDate,
		Recurrence:  t.Recurrence,
		SubTasks:    subtasksToProto(t.SubTasks),
	}
}
