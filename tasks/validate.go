package tasks

import (
	"fmt"

	"connectrpc.com/connect"

	"echolist-backend/common"
)

// validateTasks checks field sizes, counts, due_date/recurrence mutual exclusion, and RRULE validity.
func validateTasks(tasks []MainTask) error {
	if len(tasks) > common.MaxTasksPerList {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("too many tasks: %d exceeds %d limit", len(tasks), common.MaxTasksPerList))
	}
	for i, task := range tasks {
		err := common.ValidateContentLength(task.Description, common.MaxTaskDescriptionBytes, fmt.Sprintf("task %d description", i))
		if err != nil {
			return err
		}
		if len(task.SubTasks) > common.MaxSubtasksPerTask {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task %d: too many subtasks: %d exceeds %d limit", i, len(task.SubTasks), common.MaxSubtasksPerTask))
		}
		for j, subTask := range task.SubTasks {
			err := common.ValidateContentLength(subTask.Description, common.MaxSubtaskDescriptionBytes, fmt.Sprintf("task %d subtask %d description", i, j))
			if err != nil {
				return err
			}
		}
		if task.DueDate != "" && task.Recurrence != "" {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task %d: due_date must not be specified when recurrence is set (due date is computed automatically)", i))
		}
		if task.Recurrence != "" {
			err := ValidateRRule(task.Recurrence)
			if err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
		}
	}
	return nil
}
