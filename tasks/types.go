package tasks

// MainTask is the in-memory representation of a top-level task.
type MainTask struct {
	Description string
	Done        bool
	DueDate     string    // "YYYY-MM-DD" or "" (empty for simple tasks)
	Recurrence  string    // RRULE string or "" (empty for non-recurring)
	SubTasks    []SubTask
}

// SubTask is a child task under a MainTask.
type SubTask struct {
	Description string
	Done        bool
}
