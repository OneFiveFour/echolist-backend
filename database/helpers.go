package database

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// boolToInt converts a Go bool to a SQLite integer (0 or 1).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// insertTask inserts a task row and its subtasks recursively.
// taskListId is set for main tasks (parentTaskId is nil).
// parentTaskId is set for subtasks (taskListId is nil).
func insertTask(tx *sql.Tx, task CreateTaskParams, taskListId *string, parentTaskId *string, position int, rows *[]TaskRow) error {
	_, err := tx.Exec(
		`INSERT INTO tasks (id, task_list_id, parent_task_id, position, description, is_done, due_date, recurrence)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		task.Id, taskListId, parentTaskId, position,
		task.Description, boolToInt(task.IsDone),
		task.DueDate, task.Recurrence,
	)
	if err != nil {
		return fmt.Errorf("insert task %s: %w", task.Id, err)
	}

	*rows = append(*rows, TaskRow{
		Id:           task.Id,
		TaskListId:   taskListId,
		ParentTaskId: parentTaskId,
		Position:     position,
		Description:  task.Description,
		IsDone:       task.IsDone,
		DueDate:      task.DueDate,
		Recurrence:   task.Recurrence,
	})

	// Insert subtasks with parent_task_id set to this task's ID.
	for i, st := range task.SubTasks {
		if err := insertTask(tx, st, nil, &task.Id, i, rows); err != nil {
			return err
		}
	}

	return nil
}

// queryTasksByTaskListId returns all tasks (main and sub) for a task list,
// ordered so that main tasks come first by position, then subtasks by position.
func queryTasksByTaskListId(querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}, taskListId string) ([]TaskRow, error) {
	// Query all tasks that belong to this task list, either directly (main tasks)
	// or indirectly (subtasks whose parent is a main task of this list).
	sqlRows, err := querier.Query(`
		SELECT id, task_list_id, parent_task_id, position, description, is_done, due_date, recurrence
		FROM tasks
		WHERE task_list_id = ?
		   OR parent_task_id IN (SELECT id FROM tasks WHERE task_list_id = ?)
		ORDER BY
			CASE WHEN task_list_id IS NOT NULL THEN 0 ELSE 1 END,
			COALESCE(parent_task_id, task_list_id),
			position`,
		taskListId, taskListId,
	)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer sqlRows.Close()

	var rows []TaskRow
	for sqlRows.Next() {
		var r TaskRow
		var isDone int
		if err := sqlRows.Scan(&r.Id, &r.TaskListId, &r.ParentTaskId, &r.Position,
			&r.Description, &isDone, &r.DueDate, &r.Recurrence); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		r.IsDone = isDone != 0
		rows = append(rows, r)
	}
	if err := sqlRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}

	return rows, nil
}
