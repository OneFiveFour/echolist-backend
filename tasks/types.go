package tasks

// MainTask is the in-memory representation of a top-level task.
type MainTask struct {
	Id          string
	Description string
	IsDone      bool
	DueDate     string    // "YYYY-MM-DD" or "" (empty for simple tasks)
	Recurrence  string    // RRULE string or "" (empty for non-recurring)
	SubTasks    []SubTask
}

// SubTask is a child task under a MainTask.
type SubTask struct {
	Id          string
	Description string
	IsDone      bool
}
