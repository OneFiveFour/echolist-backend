package database

import (
	"database/sql"
	"fmt"
)

// CreateTaskList inserts a task list with its tasks (main and sub) in a single
// transaction. Returns the populated TaskListRow and all TaskRows.
func (d *Database) CreateTaskList(params CreateTaskListParams) (TaskListRow, []TaskRow, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return TaskListRow{}, nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO task_lists (id, title, parent_dir, is_auto_delete, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		params.Id, params.Title, params.ParentDir,
		boolToInt(params.IsAutoDelete),
		params.CreatedAt, params.UpdatedAt,
	)
	if err != nil {
		return TaskListRow{}, nil, fmt.Errorf("insert task_lists: %w", err)
	}

	var rows []TaskRow
	for i, mt := range params.Tasks {
		if err := insertTask(tx, mt, &params.Id, nil, i, &rows); err != nil {
			return TaskListRow{}, nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return TaskListRow{}, nil, fmt.Errorf("commit transaction: %w", err)
	}

	tl := TaskListRow{
		Id:           params.Id,
		Title:        params.Title,
		ParentDir:    params.ParentDir,
		IsAutoDelete: params.IsAutoDelete,
		CreatedAt:    params.CreatedAt,
		UpdatedAt:    params.UpdatedAt,
	}
	return tl, rows, nil
}

// GetTaskList retrieves a task list by ID with all tasks ordered by position.
func (d *Database) GetTaskList(id string) (TaskListRow, []TaskRow, error) {
	var tl TaskListRow
	var isAutoDelete int
	err := d.db.QueryRow(
		`SELECT id, title, parent_dir, is_auto_delete, created_at, updated_at
		 FROM task_lists WHERE id = ?`, id,
	).Scan(&tl.Id, &tl.Title, &tl.ParentDir, &isAutoDelete, &tl.CreatedAt, &tl.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return TaskListRow{}, nil, ErrNotFound
		}
		return TaskListRow{}, nil, fmt.Errorf("query task_lists: %w", err)
	}
	tl.IsAutoDelete = isAutoDelete != 0

	rows, err := queryTasksByTaskListId(d.db, id)
	if err != nil {
		return TaskListRow{}, nil, err
	}

	return tl, rows, nil
}

// UpdateTaskList replaces all tasks for a task list within a single transaction.
func (d *Database) UpdateTaskList(params UpdateTaskListParams) (TaskListRow, []TaskRow, error) {
	tx, err := d.db.Begin()
	if err != nil {
		return TaskListRow{}, nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Verify the task list exists and get current data.
	var tl TaskListRow
	var isAutoDelete int
	err = tx.QueryRow(
		`SELECT id, title, parent_dir, is_auto_delete, created_at, updated_at
		 FROM task_lists WHERE id = ?`, params.Id,
	).Scan(&tl.Id, &tl.Title, &tl.ParentDir, &isAutoDelete, &tl.CreatedAt, &tl.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return TaskListRow{}, nil, ErrNotFound
		}
		return TaskListRow{}, nil, fmt.Errorf("query task_lists: %w", err)
	}

	// Update the task list metadata.
	_, err = tx.Exec(
		`UPDATE task_lists SET title = ?, is_auto_delete = ?, updated_at = ? WHERE id = ?`,
		params.Title, boolToInt(params.IsAutoDelete), params.UpdatedAt, params.Id,
	)
	if err != nil {
		return TaskListRow{}, nil, fmt.Errorf("update task_lists: %w", err)
	}

	// Delete all tasks (main and sub) belonging to this task list.
	// Subtasks have task_list_id=NULL and reference a parent task via
	// parent_task_id, so we match both in a single statement.
	_, err = tx.Exec(
		`DELETE FROM tasks WHERE task_list_id = ? OR parent_task_id IN (SELECT id FROM tasks WHERE task_list_id = ?)`,
		params.Id, params.Id,
	)
	if err != nil {
		return TaskListRow{}, nil, fmt.Errorf("delete existing tasks: %w", err)
	}

	// Insert new tasks.
	var rows []TaskRow
	for i, mt := range params.Tasks {
		if err := insertTask(tx, mt, &params.Id, nil, i, &rows); err != nil {
			return TaskListRow{}, nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return TaskListRow{}, nil, fmt.Errorf("commit transaction: %w", err)
	}

	tl.Title = params.Title
	tl.IsAutoDelete = params.IsAutoDelete
	tl.UpdatedAt = params.UpdatedAt
	return tl, rows, nil
}

// DeleteTaskList deletes a task list by ID. Cascade foreign keys delete all
// associated tasks. Returns false if the ID was not found.
func (d *Database) DeleteTaskList(id string) (bool, error) {
	result, err := d.db.Exec(`DELETE FROM task_lists WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("delete task_lists: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n > 0, nil
}

// ListTaskLists returns all task lists in a given parent directory with their tasks.
func (d *Database) ListTaskLists(parentDir string) ([]TaskListRow, map[string][]TaskRow, error) {
	sqlRows, err := d.db.Query(
		`SELECT id, title, parent_dir, is_auto_delete, created_at, updated_at
		 FROM task_lists WHERE parent_dir = ?`, parentDir,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("query task_lists: %w", err)
	}
	defer sqlRows.Close()

	var lists []TaskListRow
	for sqlRows.Next() {
		var tl TaskListRow
		var isAutoDelete int
		if err := sqlRows.Scan(&tl.Id, &tl.Title, &tl.ParentDir, &isAutoDelete, &tl.CreatedAt, &tl.UpdatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan task_lists: %w", err)
		}
		tl.IsAutoDelete = isAutoDelete != 0
		lists = append(lists, tl)
	}
	if err := sqlRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate task_lists: %w", err)
	}

	tasksByList := make(map[string][]TaskRow, len(lists))
	for _, tl := range lists {
		tasks, err := queryTasksByTaskListId(d.db, tl.Id)
		if err != nil {
			return nil, nil, err
		}
		tasksByList[tl.Id] = tasks
	}

	return lists, tasksByList, nil
}

// GetMainTask retrieves a single main task by ID along with its subtask rows.
// Returns ErrNotFound if the ID does not exist or is not a main task.
func (d *Database) GetMainTask(id string) (TaskRow, []TaskRow, error) {
	var mainRow TaskRow
	var isDone int
	err := d.db.QueryRow(
		`SELECT id, task_list_id, parent_task_id, position, description, is_done, due_date, recurrence
		 FROM tasks WHERE id = ? AND task_list_id IS NOT NULL`, id,
	).Scan(&mainRow.Id, &mainRow.TaskListId, &mainRow.ParentTaskId, &mainRow.Position,
		&mainRow.Description, &isDone, &mainRow.DueDate, &mainRow.Recurrence)
	if err != nil {
		if err == sql.ErrNoRows {
			return TaskRow{}, nil, ErrNotFound
		}
		return TaskRow{}, nil, fmt.Errorf("query main task: %w", err)
	}
	mainRow.IsDone = isDone != 0

	// Fetch subtasks ordered by position.
	sqlRows, err := d.db.Query(
		`SELECT id, task_list_id, parent_task_id, position, description, is_done, due_date, recurrence
		 FROM tasks WHERE parent_task_id = ? ORDER BY position`, id,
	)
	if err != nil {
		return TaskRow{}, nil, fmt.Errorf("query subtasks: %w", err)
	}
	defer sqlRows.Close()

	var subRows []TaskRow
	for sqlRows.Next() {
		var r TaskRow
		var subIsDone int
		if err := sqlRows.Scan(&r.Id, &r.TaskListId, &r.ParentTaskId, &r.Position,
			&r.Description, &subIsDone, &r.DueDate, &r.Recurrence); err != nil {
			return TaskRow{}, nil, fmt.Errorf("scan subtask: %w", err)
		}
		r.IsDone = subIsDone != 0
		subRows = append(subRows, r)
	}
	if err := sqlRows.Err(); err != nil {
		return TaskRow{}, nil, fmt.Errorf("iterate subtasks: %w", err)
	}

	return mainRow, subRows, nil
}

// ListTaskListsWithCounts returns task lists in a parent directory with
// aggregate task counts populated. Used by FileServer.ListFiles.
func (d *Database) ListTaskListsWithCounts(parentDir string) ([]TaskListRow, error) {
	sqlRows, err := d.db.Query(`
		SELECT tl.id, tl.title, tl.parent_dir, tl.is_auto_delete, tl.created_at, tl.updated_at,
		       COUNT(t.id) AS total_task_count,
		       SUM(CASE WHEN t.is_done = 1 THEN 1 ELSE 0 END) AS done_task_count
		FROM task_lists tl
		LEFT JOIN tasks t ON t.task_list_id = tl.id
		WHERE tl.parent_dir = ?
		GROUP BY tl.id`, parentDir,
	)
	if err != nil {
		return nil, fmt.Errorf("query task_lists with counts: %w", err)
	}
	defer sqlRows.Close()

	var lists []TaskListRow
	for sqlRows.Next() {
		var tl TaskListRow
		var isAutoDelete int
		var doneCount sql.NullInt64
		if err := sqlRows.Scan(&tl.Id, &tl.Title, &tl.ParentDir, &isAutoDelete,
			&tl.CreatedAt, &tl.UpdatedAt, &tl.TotalTaskCount, &doneCount); err != nil {
			return nil, fmt.Errorf("scan task_lists with counts: %w", err)
		}
		tl.IsAutoDelete = isAutoDelete != 0
		if doneCount.Valid {
			tl.DoneTaskCount = int(doneCount.Int64)
		}
		lists = append(lists, tl)
	}
	if err := sqlRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task_lists with counts: %w", err)
	}

	return lists, nil
}
