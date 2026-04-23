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

	title := req.GetTitle()
	if err := common.ValidateName(title); err != nil {
		return nil, err
	}

	domainTasks := protoToMainTasks(req.GetTasks())
	if err := validateTasks(domainTasks); err != nil {
		return nil, err
	}

	for i, t := range domainTasks {
		if t.Recurrence != "" {
			next, err := ComputeNextDueDate(t.Recurrence, time.Now())
			if err != nil {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			domainTasks[i].DueDate = next.Format("2006-01-02")
		}
	}

	if err := common.RequireDir(dirPath, "parent directory"); err != nil {
		return nil, err
	}

	id := uuid.NewString()

	taskParams := make([]database.CreateTaskParams, len(domainTasks))
	for i, mt := range domainTasks {
		mtId := uuid.NewString()
		domainTasks[i].Id = mtId

		subParams := make([]database.CreateTaskParams, len(mt.SubTasks))
		for j, st := range mt.SubTasks {
			stId := uuid.NewString()
			domainTasks[i].SubTasks[j].Id = stId

			subParams[j] = database.CreateTaskParams{
				Id:          stId,
				Description: st.Description,
				IsDone:      st.IsDone,
			}
		}

		var dueDate *string
		if mt.DueDate != "" {
			d := domainTasks[i].DueDate
			dueDate = &d
		}
		var recurrence *string
		if mt.Recurrence != "" {
			r := mt.Recurrence
			recurrence = &r
		}

		taskParams[i] = database.CreateTaskParams{
			Id:          mtId,
			Description: mt.Description,
			IsDone:      mt.IsDone,
			DueDate:     dueDate,
			Recurrence:  recurrence,
			SubTasks:    subParams,
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
