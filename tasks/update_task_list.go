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
	// Validate ID
	id := req.GetId()
	err := common.ValidateUuidV4(id)
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

	// Validate and assign IDs for main tasks and subtasks.
	for i, mainTask := range domainTasks {
		if mainTask.Id != "" {
			err = common.ValidateUuidV4(mainTask.Id)
			if err != nil {
				return nil, err
			}
		} else {
			domainTasks[i].Id = uuid.NewString()
		}
		for j, subTask := range mainTask.SubTasks {
			if subTask.Id != "" {
				err = common.ValidateUuidV4(subTask.Id)
				if err != nil {
					return nil, err
				}
			} else {
				domainTasks[i].SubTasks[j].Id = uuid.NewString()
			}
		}
	}

	// Get existing tasks from DB for recurrence advancement reference.
	_, existingRows, err := s.db.GetTaskList(id)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		s.logger.Error("failed to get task list", "id", id, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get task list: %w", err))
	}
	existingTasks := taskRowsToMainTasks(existingRows)

	err = advanceRecurringTasks(domainTasks, existingTasks)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to compute next due date: %w", err))
	}

	isAutoDelete := req.GetIsAutoDelete()

	// Apply AutoDelete filtering after recurrence advancement.
	if isAutoDelete {
		domainTasks = filterAutoDeleted(domainTasks)
	}

	// Build CreateTaskParams for the database update.
	taskParams := make([]database.CreateTaskParams, len(domainTasks))
	for i, mainTask := range domainTasks {
		subTaskParams := make([]database.CreateTaskParams, len(mainTask.SubTasks))
		for j, subTask := range mainTask.SubTasks {
			subTaskParams[j] = database.CreateTaskParams{
				Id:          subTask.Id,
				Description: subTask.Description,
				IsDone:      subTask.IsDone,
			}
		}

		var dueDate *string
		if mainTask.DueDate != "" {
			dueDateValue := mainTask.DueDate
			dueDate = &dueDateValue
		}
		var recurrence *string
		if mainTask.Recurrence != "" {
			recurrenceValue := mainTask.Recurrence
			recurrence = &recurrenceValue
		}

		taskParams[i] = database.CreateTaskParams{
			Id:          mainTask.Id,
			Description: mainTask.Description,
			IsDone:      mainTask.IsDone,
			DueDate:     dueDate,
			Recurrence:  recurrence,
			SubTasks:    subTaskParams,
		}
	}

	tlRow, _, err := s.db.UpdateTaskList(database.UpdateTaskListParams{
		Id:           id,
		Title:        title,
		IsAutoDelete: isAutoDelete,
		UpdatedAt:    nowMillis(),
		Tasks:        taskParams,
	})
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task list not found"))
		}
		s.logger.Error("failed to update task list", "id", id, "error", err)
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
	for _, existingTask := range existingTasks {
		if existingTask.Recurrence != "" {
			existingByKey[taskKey{existingTask.Description, existingTask.Recurrence}] = existingTask
		}
	}

	for i, task := range domainTasks {
		if task.Recurrence == "" || !task.IsDone {
			continue
		}
		var prevDue string
		if existingTask, ok := existingByKey[taskKey{task.Description, task.Recurrence}]; ok {
			prevDue = existingTask.DueDate
		} else {
			prevDue = task.DueDate
		}

		after := time.Now()
		if prevDue != "" {
			if parsed, err := time.Parse("2006-01-02", prevDue); err == nil {
				after = parsed
			}
		}

		next, err := ComputeNextDueDate(task.Recurrence, after)
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
	for _, mainTask := range tasks {
		if mainTask.IsDone && mainTask.Recurrence == "" {
			continue
		}
		filtered := MainTask{
			Id:          mainTask.Id,
			Description: mainTask.Description,
			IsDone:      mainTask.IsDone,
			DueDate:     mainTask.DueDate,
			Recurrence:  mainTask.Recurrence,
		}
		for _, subTask := range mainTask.SubTasks {
			if !subTask.IsDone {
				filtered.SubTasks = append(filtered.SubTasks, subTask)
			}
		}
		result = append(result, filtered)
	}
	return result
}
