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
	for i, t := range tasks {
		if err := common.ValidateContentLength(t.Description, common.MaxTaskDescriptionBytes, fmt.Sprintf("task %d description", i)); err != nil {
			return err
		}
		if len(t.SubTasks) > common.MaxSubtasksPerTask {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task %d: too many subtasks: %d exceeds %d limit", i, len(t.SubTasks), common.MaxSubtasksPerTask))
		}
		for j, st := range t.SubTasks {
			if err := common.ValidateContentLength(st.Description, common.MaxSubtaskDescriptionBytes, fmt.Sprintf("task %d subtask %d description", i, j)); err != nil {
				return err
			}
		}
		if t.DueDate != "" && t.Recurrence != "" {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task %d: due_date must not be specified when recurrence is set (due date is computed automatically)", i))
		}
		if t.Recurrence != "" {
			if err := ValidateRRule(t.Recurrence); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
		}
	}
	return nil
}
