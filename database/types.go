package database

// TaskListRow represents a row from the task_lists table.
type TaskListRow struct {
	Id             string
	Title          string
	ParentDir      string // "" for root
	IsAutoDelete   bool
	CreatedAt      int64
	UpdatedAt      int64
	TotalTaskCount int // populated by aggregate queries, zero otherwise
	DoneTaskCount  int // populated by aggregate queries, zero otherwise
}

// TaskRow represents a row from the tasks table.
// A main task has TaskListId set and ParentTaskId nil.
// A subtask has ParentTaskId set and TaskListId nil.
type TaskRow struct {
	Id           string
	TaskListId   *string // non-nil for main tasks, nil for subtasks
	ParentTaskId *string // non-nil for subtasks, nil for main tasks
	Position     int
	Description  string
	IsDone       bool
	DueDate      *string // nil when no due date
	Recurrence   *string // nil when no recurrence
}

// NoteRow represents a row from the notes table.
type NoteRow struct {
	Id        string
	Title     string
	ParentDir string // "" for root
	Preview   string
	CreatedAt int64
	UpdatedAt int64
}

// CreateTaskListParams holds the parameters for creating a new task list.
type CreateTaskListParams struct {
	Id           string
	Title        string
	ParentDir    string // "" for root
	IsAutoDelete bool
	CreatedAt    int64
	UpdatedAt    int64
	Tasks        []CreateTaskParams
}

// CreateTaskParams holds the parameters for creating a task (main or sub).
type CreateTaskParams struct {
	Id          string
	Description string
	IsDone      bool
	DueDate     *string
	Recurrence  *string
	SubTasks    []CreateTaskParams
}

// UpdateTaskListParams holds the parameters for updating a task list.
type UpdateTaskListParams struct {
	Id           string
	Title        string
	IsAutoDelete bool
	UpdatedAt    int64
	Tasks        []CreateTaskParams
}

// InsertNoteParams holds the parameters for inserting a note metadata row.
type InsertNoteParams struct {
	Id        string
	Title     string
	ParentDir string // "" for root
	Preview   string
	CreatedAt int64
	UpdatedAt int64
}
