package tasks

import (
	"fmt"
	"regexp"

	"connectrpc.com/connect"

	"echolist-backend/common"
)

// dueDateRegex matches the YYYY-MM-DD date format.
var dueDateRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// validateTasks checks field sizes, counts, due_date format, recurrence validity,
// and that recurrence requires a due_date.
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
		if task.DueDate != "" && !dueDateRegex.MatchString(task.DueDate) {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task %d: due_date must match YYYY-MM-DD format", i))
		}
		if task.Recurrence != "" {
			if task.DueDate == "" {
				return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task %d: due_date is required when recurrence is set", i))
			}
			err := ValidateRRule(task.Recurrence)
			if err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
		}
	}
	return nil
}
