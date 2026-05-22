package tasks

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"echolist-backend/common"
	"echolist-backend/database"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) CreateTaskList(
	ctx context.Context,
	req *pb.CreateTaskListRequest,
) (*pb.CreateTaskListResponse, error) {
	parentDir := req.GetParentDir()

	dirPath, err := common.ValidateParentDir(s.dataDir, parentDir)
	if err != nil {
		return nil, err
	}

	// Validate title
	title := req.GetTitle()
	err = common.ValidateName(title)
	if err != nil {
		return nil, err
	}

	// Validate tasks
	domainTasks := protoToMainTasks(req.GetTasks())
	err = validateTasks(domainTasks)
	if err != nil {
		return nil, err
	}

	for i, task := range domainTasks {
		if task.Recurrence != "" {
			next, err := ComputeNextDueDate(task.Recurrence, time.Now())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			domainTasks[i].DueDate = next.Format("2006-01-02")
		}
	}

	// Validate parent directory exists
	err = common.RequireDir(dirPath)
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()

	taskParams := make([]database.CreateTaskParams, len(domainTasks))
	for i, mainTask := range domainTasks {
		mainTaskId := uuid.NewString()
		domainTasks[i].Id = mainTaskId

		subTaskParams := make([]database.CreateTaskParams, len(mainTask.SubTasks))
		for j, subTask := range mainTask.SubTasks {
			subTaskId := uuid.NewString()
			domainTasks[i].SubTasks[j].Id = subTaskId

			subTaskParams[j] = database.CreateTaskParams{
				Id:          subTaskId,
				Description: subTask.Description,
				IsDone:      subTask.IsDone,
			}
		}

		var dueDate *string
		if mainTask.DueDate != "" {
			dueDateValue := domainTasks[i].DueDate
			dueDate = &dueDateValue
		}
		var recurrence *string
		if mainTask.Recurrence != "" {
			recurrenceValue := mainTask.Recurrence
			recurrence = &recurrenceValue
		}

		taskParams[i] = database.CreateTaskParams{
			Id:          mainTaskId,
			Description: mainTask.Description,
			IsDone:      mainTask.IsDone,
			DueDate:     dueDate,
			Recurrence:  recurrence,
			SubTasks:    subTaskParams,
		}
	}

	now := nowMillis()
	isAutoDelete := req.GetIsAutoDelete()

	tlRow, _, err := s.db.CreateTaskList(database.CreateTaskListParams{
		Id:           id,
		Title:        title,
		ParentDir:    parentDir,
		IsAutoDelete: isAutoDelete,
		CreatedAt:    now,
		UpdatedAt:    now,
		Tasks:        taskParams,
	})
	if err != nil {
		s.logger.Error("failed to create task list", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create task list: %w", err))
	}

	return &pb.CreateTaskListResponse{
		TaskList: buildTaskList(tlRow.Id, tlRow.ParentDir, tlRow.Title, domainTasks, tlRow.UpdatedAt, tlRow.IsAutoDelete),
	}, nil
}
