package tasks

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// subtaskGen generates a random SubTask with a non-empty description.
func subtaskGen() *rapid.Generator[SubTask] {
	return rapid.Custom[SubTask](func(t *rapid.T) SubTask {
		desc := rapid.StringMatching(`[A-Za-z0-9 ]{1,40}`).Draw(t, "subtask-desc")
		return SubTask{
			Description: desc,
			Done:        rapid.Bool().Draw(t, "subtask-done"),
		}
	})
}

// mainTaskGen generates a random MainTask in one of three modes (simple, deadline, recurring).
// Descriptions never contain "|" or newlines since "|" is the metadata delimiter.
func mainTaskGen() *rapid.Generator[MainTask] {
	return rapid.Custom[MainTask](func(t *rapid.T) MainTask {
		desc := rapid.StringMatching(`[A-Za-z0-9 ]{1,40}`).Draw(t, "main-desc")
		done := rapid.Bool().Draw(t, "main-done")
		subtasks := rapid.SliceOfN(subtaskGen(), 0, 5).Draw(t, "subtasks")
		if len(subtasks) == 0 {
			subtasks = nil
		}

		mode := rapid.IntRange(0, 2).Draw(t, "mode")
		var dueDate, recurrence string
		switch mode {
		case 1: // deadline
			dueDate = validDueDateGen().Draw(t, "due-date")
		case 2: // recurring
			dueDate = validDueDateGen().Draw(t, "rec-due-date")
			recurrence = validRRuleGen().Draw(t, "recurrence")
		}

		return MainTask{
			Description: desc,
			Done:        done,
			DueDate:     dueDate,
			Recurrence:  recurrence,
			SubTasks:    subtasks,
		}
	})
}

// taskListGen generates a slice of 0-10 MainTask values.
func taskListGen() *rapid.Generator[[]MainTask] {
	return rapid.Custom[[]MainTask](func(t *rapid.T) []MainTask {
		tasks := rapid.SliceOfN(mainTaskGen(), 0, 10).Draw(t, "tasks")
		if len(tasks) == 0 {
			return nil
		}
		return tasks
	})
}

// validDueDateGen generates dates in YYYY-MM-DD format.
func validDueDateGen() *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		year := rapid.IntRange(2020, 2035).Draw(t, "year")
		month := rapid.IntRange(1, 12).Draw(t, "month")
		day := rapid.IntRange(1, 28).Draw(t, "day") // 28 to avoid invalid dates
		return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	})
}

// validRRuleGen generates valid RRULE strings from a supported subset.
func validRRuleGen() *rapid.Generator[string] {
	return rapid.Custom[string](func(t *rapid.T) string {
		rules := []string{
			"FREQ=DAILY",
			"FREQ=WEEKLY",
			"FREQ=MONTHLY",
			"FREQ=YEARLY",
			"FREQ=DAILY;INTERVAL=2",
			"FREQ=DAILY;INTERVAL=3",
			"FREQ=WEEKLY;BYDAY=MO",
			"FREQ=WEEKLY;BYDAY=TU",
			"FREQ=WEEKLY;BYDAY=FR",
			"FREQ=MONTHLY;BYDAY=1MO",
		}
		return rapid.SampledFrom(rules).Draw(t, "rrule")
	})
}

// Feature: task-management, Property 1: Task file parse/print round-trip
// **Validates: Requirements 7.2, 7.3, 7.4, 7.5, 7.7**
func TestProperty1_TaskFileRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		original := taskListGen().Draw(rt, "tasks")

		// Print → Parse
		printed := PrintTaskFile(original)
		parsed, err := ParseTaskFile(printed)
		if err != nil {
			rt.Fatalf("ParseTaskFile failed on printed output: %v\nprinted:\n%s", err, string(printed))
		}

		// Parsed tasks must equal original
		if !reflect.DeepEqual(normalizeNilSlices(original), normalizeNilSlices(parsed)) {
			rt.Fatalf("round-trip mismatch:\noriginal: %+v\nparsed:   %+v\nprinted:\n%s", original, parsed, string(printed))
		}

		// Print the parsed result — must be byte-identical to first print
		reprinted := PrintTaskFile(parsed)
		if string(printed) != string(reprinted) {
			rt.Fatalf("reprint mismatch:\nfirst:  %q\nsecond: %q", string(printed), string(reprinted))
		}
	})
}

// normalizeNilSlices ensures nil and empty slices compare equal for DeepEqual.
func normalizeNilSlices(tasks []MainTask) []MainTask {
	if tasks == nil {
		return nil
	}
	result := make([]MainTask, len(tasks))
	for i, mt := range tasks {
		result[i] = mt
		if result[i].SubTasks == nil {
			result[i].SubTasks = nil
		}
	}
	return result
}

// Feature: task-management, Property 14: Malformed task file produces parse error with line number
// **Validates: Requirements 7.6**
func TestProperty14_MalformedInputParseError(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate lines that are NOT valid task lines
		numLines := rapid.IntRange(1, 5).Draw(rt, "num-lines")
		var lines []string
		for i := 0; i < numLines; i++ {
			line := rapid.SampledFrom([]string{
				"not a task",
				"  indented but not a task",
				"- [] missing space",
				"- [X] wrong case",
				"--- invalid prefix",
				"* bullet point",
				"  * indented bullet",
				"random text here",
				"123 numbers only",
				"  - [] missing space in subtask",
			}).Draw(rt, fmt.Sprintf("line-%d", i))
			lines = append(lines, line)
		}

		input := []byte(strings.Join(lines, "\n"))
		_, err := ParseTaskFile(input)
		if err == nil {
			rt.Fatalf("expected parse error for malformed input %q, got nil", string(input))
		}

		// Error must contain a line number
		errMsg := err.Error()
		if !strings.Contains(errMsg, "line ") {
			rt.Fatalf("error message %q does not contain line number", errMsg)
		}
	})
}
