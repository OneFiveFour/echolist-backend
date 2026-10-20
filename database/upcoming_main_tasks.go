package database

import (
	"context"
	"fmt"
)

// UpcomingMainTaskRow contains only the fields needed for a task notification.
type UpcomingMainTaskRow struct {
	TaskId     string
	TaskListId string
	Title      string
	DueDate    string
}

// ListUpcomingMainTasks returns uncompleted main tasks across all folders with
// a due date on or after fromDate. The caller must supply a valid YYYY-MM-DD UTC
// date. Dates in this format sort chronologically without timezone conversion.
func (d *Database) ListUpcomingMainTasks(ctx context.Context, fromDate string) ([]UpcomingMainTaskRow, error) {
	sqlRows, err := d.db.QueryContext(ctx, `
		SELECT id, task_list_id, description, due_date
		FROM tasks
		WHERE task_list_id IS NOT NULL
		  AND is_done = 0
		  AND due_date >= ?
		ORDER BY due_date, id`, fromDate)
	if err != nil {
		return nil, fmt.Errorf("query upcoming main tasks: %w", err)
	}
	defer sqlRows.Close()

	rows := make([]UpcomingMainTaskRow, 0)
	for sqlRows.Next() {
		var row UpcomingMainTaskRow
		if err := sqlRows.Scan(&row.TaskId, &row.TaskListId, &row.Title, &row.DueDate); err != nil {
			return nil, fmt.Errorf("scan upcoming main task: %w", err)
		}
		rows = append(rows, row)
	}
	if err := sqlRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate upcoming main tasks: %w", err)
	}
	return rows, nil
}
