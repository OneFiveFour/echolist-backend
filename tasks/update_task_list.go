package tasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"echolist-backend/common"
	"echolist-backend/database"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func (s *TaskServer) UpdateTaskList(
	ctx context.Context,
	req *pb.UpdateTaskListRequest,
) (*pb.UpdateTaskListResponse, error) {
	if err := common.ValidateUuidV4(req.GetId()); err != nil {
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

	// Validate and assign IDs for main tasks and subtasks.
	for i, mt := range domainTasks {
		if mt.Id != "" {
			if err := common.ValidateUuidV4(mt.Id); err != nil {
				return nil, err
			}
		} else {
			domainTasks[i].Id = uuid.NewString()
		}
		for j, st := range mt.SubTasks {
			if st.Id != "" {
				if err := common.ValidateUuidV4(st.Id); err != nil {
					return nil, err
				}
			} else {
				domainTasks[i].SubTasks[j].Id = uuid.NewString()
			}
		}
	}

	// Get existing tasks from DB for recurrence advancement reference.
	_, existingRows, err := s.db.GetTaskList(req.GetId())
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		s.logger.Error("failed to get task list", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get task list: %w", err))
	}
	existingTasks := taskRowsToMainTasks(existingRows)

	if err := advanceRecurringTasks(domainTasks, existingTasks); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to compute next due date: %w", err))
	}

	isAutoDelete := req.GetIsAutoDelete()

	// Apply AutoDelete filtering after recurrence advancement.
	if isAutoDelete {
		domainTasks = filterAutoDeleted(domainTasks)
	}

	// Build CreateTaskParams for the database update.
	taskParams := make([]database.CreateTaskParams, len(domainTasks))
	for i, mt := range domainTasks {
		subParams := make([]database.CreateTaskParams, len(mt.SubTasks))
		for j, st := range mt.SubTasks {
			subParams[j] = database.CreateTaskParams{
				Id:          st.Id,
				Description: st.Description,
				IsDone:      st.IsDone,
			}
		}

		var dueDate *string
		if mt.DueDate != "" {
			d := mt.DueDate
			dueDate = &d
		}
		var recurrence *string
		if mt.Recurrence != "" {
			r := mt.Recurrence
			recurrence = &r
		}

		taskParams[i] = database.CreateTaskParams{
			Id:          mt.Id,
			Description: mt.Description,
			IsDone:      mt.IsDone,
			DueDate:     dueDate,
			Recurrence:  recurrence,
			SubTasks:    subParams,
		}
	}

	tlRow, _, err := s.db.UpdateTaskList(database.UpdateTaskListParams{
		Id:           req.GetId(),
		Title:        title,
		IsAutoDelete: isAutoDelete,
		UpdatedAt:    nowMillis(),
		Tasks:        taskParams,
	})
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		s.logger.Error("failed to update task list", "id", req.GetId(), "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update task list: %w", err))
	}

	return &pb.UpdateTaskListResponse{
		TaskList: buildTaskList(tlRow.Id, tlRow.ParentDir, tlRow.Title, domainTasks, tlRow.UpdatedAt, tlRow.IsAutoDelete),
	}, nil
}

// advanceRecurringTasks resets done recurring tasks and computes their next due
// date based on the recurrence rule. existingTasks is used to look up the
// previous due date for matching recurring tasks. domainTasks is modified in place.
func advanceRecurringTasks(domainTasks, existingTasks []MainTask) error {
	type taskKey struct{ desc, recurrence string }
	existingByKey := make(map[taskKey]MainTask, len(existingTasks))
	for _, et := range existingTasks {
		if et.Recurrence != "" {
			existingByKey[taskKey{et.Description, et.Recurrence}] = et
		}
	}

	for i, t := range domainTasks {
		if t.Recurrence == "" || !t.IsDone {
			continue
		}
		var prevDue string
		if et, ok := existingByKey[taskKey{t.Description, t.Recurrence}]; ok {
			prevDue = et.DueDate
		} else {
			prevDue = t.DueDate
		}

		after := time.Now()
		if prevDue != "" {
			if parsed, err := time.Parse("2006-01-02", prevDue); err == nil {
				after = parsed
			}
		}

		next, err := ComputeNextDueDate(t.Recurrence, after)
		if err != nil {
			return err
		}
		domainTasks[i].IsDone = false
		domainTasks[i].DueDate = next.Format("2006-01-02")
	}
	return nil
}

// filterAutoDeleted removes done tasks from the list when AutoDelete is enabled.
// It removes MainTasks where IsDone == true and Recurrence == "" (non-recurring),
// along with all their SubTasks. For surviving MainTasks, it removes SubTasks
// where IsDone == true. Returns a new slice; does not mutate the input.
func filterAutoDeleted(tasks []MainTask) []MainTask {
	var result []MainTask
	for _, mt := range tasks {
		if mt.IsDone && mt.Recurrence == "" {
			continue
		}
		filtered := MainTask{
			Id:          mt.Id,
			Description: mt.Description,
			IsDone:      mt.IsDone,
			DueDate:     mt.DueDate,
			Recurrence:  mt.Recurrence,
		}
		for _, st := range mt.SubTasks {
			if !st.IsDone {
				filtered.SubTasks = append(filtered.SubTasks, st)
			}
		}
		result = append(result, filtered)
	}
	return result
}
