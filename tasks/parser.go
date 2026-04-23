package tasks

import (
	"fmt"
	"strings"
)

// ParseTaskFile parses a task file's byte content into a list of MainTask values.
//
// Parsing rules:
//   - Lines starting with "- [ ] " or "- [x] " at column 0 are main tasks
//   - Lines starting with "  - [ ] " or "  - [x] " (2-space indent) are subtasks
//   - After the description, an optional " | " delimiter separates metadata fields
//   - Metadata fields: "due:YYYY-MM-DD", "recurrence:RRULE_STRING"
//   - Blank lines are ignored
//   - Any other format produces a parse error with line number
func ParseTaskFile(data []byte) ([]MainTask, error) {
	if len(data) == 0 {
		return nil, nil
	}

	lines := strings.Split(string(data), "\n")
	var tasks []MainTask

	for i, line := range lines {
		lineNum := i + 1

		// Ignore blank lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Try subtask first (2-space indent)
		if strings.HasPrefix(line, "  - [ ] ") || strings.HasPrefix(line, "  - [x] ") {
			if len(tasks) == 0 {
				return nil, fmt.Errorf("line %d: subtask without a preceding main task", lineNum)
			}
			st, err := parseSubtask(line, lineNum)
			if err != nil {
				return nil, err
			}
			tasks[len(tasks)-1].SubTasks = append(tasks[len(tasks)-1].SubTasks, st)
			continue
		}

		// Try main task (column 0)
		if strings.HasPrefix(line, "- [ ] ") || strings.HasPrefix(line, "- [x] ") {
			mt, err := parseMainTask(line, lineNum)
			if err != nil {
				return nil, err
			}
			tasks = append(tasks, mt)
			continue
		}

		return nil, fmt.Errorf("line %d: expected task or subtask, got %q", lineNum, line)
	}

	return tasks, nil
}

func parseMainTask(line string, lineNum int) (MainTask, error) {
	var mt MainTask

	if strings.HasPrefix(line, "- [x] ") {
		mt.IsDone = true
		line = line[6:] // len("- [x] ") == 6
	} else {
		line = line[6:] // len("- [ ] ") == 6
	}

	mt.Description, mt.DueDate, mt.Recurrence = parseDescriptionAndMetadata(line)

	if mt.Description == "" {
		return MainTask{}, fmt.Errorf("line %d: main task has empty description", lineNum)
	}

	return mt, nil
}

func parseSubtask(line string, lineNum int) (SubTask, error) {
	var st SubTask

	if strings.HasPrefix(line, "  - [x] ") {
		st.IsDone = true
		line = line[8:] // len("  - [x] ") == 8
	} else {
		line = line[8:] // len("  - [ ] ") == 8
	}

	st.Description = line

	if st.Description == "" {
		return SubTask{}, fmt.Errorf("line %d: subtask has empty description", lineNum)
	}

	return st, nil
}

// parseDescriptionAndMetadata splits a line (after the checkbox prefix) into
// description, due date, and recurrence fields using " | " as the delimiter.
func parseDescriptionAndMetadata(s string) (description, dueDate, recurrence string) {
	parts := strings.Split(s, " | ")
	description = parts[0]

	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "due:") {
			dueDate = part[4:] // len("due:") == 4
		} else if strings.HasPrefix(part, "recurrence:") {
			recurrence = part[11:] // len("recurrence:") == 11
		}
	}

	return description, dueDate, recurrence
}
