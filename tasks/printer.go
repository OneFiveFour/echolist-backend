package tasks

import "bytes"

// PrintTaskFile serializes a list of MainTask values into the task file format.
// Output rules:
//   - Main tasks: "- [ ] Description" or "- [x] Description"
//   - Deadline tasks append " | due:YYYY-MM-DD"
//   - Recurring tasks append " | due:YYYY-MM-DD | recurrence:RRULE_STRING"
//   - Subtasks: 2-space indent "  - [ ] Description"
//   - No trailing newline after last task, single newline between tasks (no blank lines)
func PrintTaskFile(tasks []MainTask) []byte {
	var buf bytes.Buffer
	for i, mt := range tasks {
		if i > 0 {
			buf.WriteByte('\n')
		}
		printMainTask(&buf, mt)
		for _, st := range mt.SubTasks {
			buf.WriteByte('\n')
			printSubtask(&buf, st)
		}
	}
	return buf.Bytes()
}

func printMainTask(buf *bytes.Buffer, mt MainTask) {
	if mt.Done {
		buf.WriteString("- [x] ")
	} else {
		buf.WriteString("- [ ] ")
	}
	buf.WriteString(mt.Description)
	if mt.DueDate != "" {
		buf.WriteString(" | due:")
		buf.WriteString(mt.DueDate)
	}
	if mt.Recurrence != "" {
		buf.WriteString(" | recurrence:")
		buf.WriteString(mt.Recurrence)
	}
}

func printSubtask(buf *bytes.Buffer, st SubTask) {
	if st.Done {
		buf.WriteString("  - [x] ")
	} else {
		buf.WriteString("  - [ ] ")
	}
	buf.WriteString(st.Description)
}
