package tasks

// MainTask is the in-memory representation of a top-level task.
type MainTask struct {
	Description string
	Done        bool
	DueDate     string    // "YYYY-MM-DD" or "" (empty for simple tasks)
	Recurrence  string    // RRULE string or "" (empty for non-recurring)
	Subtasks    []Subtask
}

// Subtask is a child task under a MainTask.
type Subtask struct {
	Description string
	Done        bool
}
