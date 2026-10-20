package tasks

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) ListUpcomingMainTasks(
	ctx context.Context,
	req *pb.ListUpcomingMainTasksRequest,
) (*pb.ListUpcomingMainTasksResponse, error) {
	fromDate := req.GetFromDate()
	if _, err := time.Parse(time.DateOnly, fromDate); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("from_date must be a valid UTC date in YYYY-MM-DD format"))
	}

	rows, err := s.db.ListUpcomingMainTasks(ctx, fromDate)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, connect.NewError(connect.CodeCanceled, context.Canceled)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, connect.NewError(connect.CodeDeadlineExceeded, context.DeadlineExceeded)
		}
		s.logger.Error("failed to list upcoming main tasks", "error", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list upcoming main tasks"))
	}

	tasks := make([]*pb.UpcomingMainTask, len(rows))
	for i, row := range rows {
		tasks[i] = &pb.UpcomingMainTask{
			TaskId:  row.TaskId,
			Title:   row.Title,
			DueDate: row.DueDate,
		}
	}
	return &pb.ListUpcomingMainTasksResponse{Tasks: tasks}, nil
}
