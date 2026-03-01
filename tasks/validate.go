package tasks

import (
	"fmt"

	"connectrpc.com/connect"
)

// validateTasks checks due_date/recurrence mutual exclusion and RRULE validity.
func validateTasks(tasks []MainTask) error {
	for i, t := range tasks {
		if t.DueDate != "" && t.Recurrence != "" {
			return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task %d: cannot set both due_date and recurrence", i))
		}
		if t.Recurrence != "" {
			if err := ValidateRRule(t.Recurrence); err != nil {
				return connect.NewError(connect.CodeInvalidArgument, err)
			}
		}
	}
	return nil
}
