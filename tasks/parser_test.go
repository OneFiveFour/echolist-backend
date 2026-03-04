package tasks

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseTaskFile_EmptyFile(t *testing.T) {
	tasks, err := ParseTaskFile([]byte{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tasks != nil {
		t.Fatalf("expected nil, got %+v", tasks)
	}
}

func TestParseTaskFile_SingleSimpleTask(t *testing.T) {
	input := []byte("- [ ] Buy groceries")
	tasks, err := ParseTaskFile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []MainTask{
		{Description: "Buy groceries", Done: false},
	}
	if !reflect.DeepEqual(tasks, expected) {
		t.Fatalf("got %+v, want %+v", tasks, expected)
	}
}

func TestParseTaskFile_SingleDoneTask(t *testing.T) {
	input := []byte("- [x] Clean kitchen")
	tasks, err := ParseTaskFile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || !tasks[0].Done || tasks[0].Description != "Clean kitchen" {
		t.Fatalf("unexpected result: %+v", tasks)
	}
}

func TestParseTaskFile_TaskWithSubtasks(t *testing.T) {
	input := []byte("- [ ] Buy groceries\n  - [ ] Whole milk 2L\n  - [x] Bread")
	tasks, err := ParseTaskFile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []MainTask{
		{
			Description: "Buy groceries",
			Done:        false,
			SubTasks: []SubTask{
				{Description: "Whole milk 2L", Done: false},
				{Description: "Bread", Done: true},
			},
		},
	}
	if !reflect.DeepEqual(tasks, expected) {
		t.Fatalf("got %+v, want %+v", tasks, expected)
	}
}

func TestParseTaskFile_DeadlineTask(t *testing.T) {
	input := []byte("- [ ] Submit report | due:2025-02-28")
	tasks, err := ParseTaskFile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []MainTask{
		{Description: "Submit report", DueDate: "2025-02-28"},
	}
	if !reflect.DeepEqual(tasks, expected) {
		t.Fatalf("got %+v, want %+v", tasks, expected)
	}
}

func TestParseTaskFile_RecurringTask(t *testing.T) {
	input := []byte("- [ ] Buy milk | due:2025-07-21 | recurrence:FREQ=WEEKLY;BYDAY=MO")
	tasks, err := ParseTaskFile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []MainTask{
		{Description: "Buy milk", DueDate: "2025-07-21", Recurrence: "FREQ=WEEKLY;BYDAY=MO"},
	}
	if !reflect.DeepEqual(tasks, expected) {
		t.Fatalf("got %+v, want %+v", tasks, expected)
	}
}

func TestParseTaskFile_MixedModes(t *testing.T) {
	input := []byte(`- [ ] Submit report | due:2025-02-28
- [ ] Buy milk | due:2025-07-21 | recurrence:FREQ=WEEKLY;BYDAY=MO
  - [ ] Whole milk
  - [ ] Oat milk
- [ ] Walk the dog
- [x] Pay rent | due:2025-07-01 | recurrence:FREQ=MONTHLY`)
	tasks, err := ParseTaskFile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []MainTask{
		{Description: "Submit report", DueDate: "2025-02-28"},
		{
			Description: "Buy milk", DueDate: "2025-07-21", Recurrence: "FREQ=WEEKLY;BYDAY=MO",
			SubTasks: []SubTask{
				{Description: "Whole milk"},
				{Description: "Oat milk"},
			},
		},
		{Description: "Walk the dog"},
		{Description: "Pay rent", Done: true, DueDate: "2025-07-01", Recurrence: "FREQ=MONTHLY"},
	}
	if !reflect.DeepEqual(tasks, expected) {
		t.Fatalf("got %+v, want %+v", tasks, expected)
	}
}

func TestParseTaskFile_BlankLinesIgnored(t *testing.T) {
	input := []byte("- [ ] Task one\n\n- [ ] Task two\n\n")
	tasks, err := ParseTaskFile(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Description != "Task one" || tasks[1].Description != "Task two" {
		t.Fatalf("unexpected descriptions: %+v", tasks)
	}
}

func TestParseTaskFile_MalformedLine(t *testing.T) {
	input := []byte("not a task")
	_, err := ParseTaskFile(input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("error should contain line number, got: %v", err)
	}
}

func TestParseTaskFile_SubtaskWithoutMainTask(t *testing.T) {
	input := []byte("  - [ ] Orphan subtask")
	_, err := ParseTaskFile(input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("error should contain line number, got: %v", err)
	}
}

func TestParseTaskFile_MalformedOnSecondLine(t *testing.T) {
	input := []byte("- [ ] Valid task\nbad line")
	_, err := ParseTaskFile(input)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error should reference line 2, got: %v", err)
	}
}

func TestPrintTaskFile_RoundTrip(t *testing.T) {
	tasks := []MainTask{
		{
			Description: "Buy groceries",
			SubTasks: []SubTask{
				{Description: "Whole milk 2L"},
				{Description: "Bread", Done: true},
			},
		},
		{Description: "Clean kitchen", Done: true},
	}
	printed := PrintTaskFile(tasks)
	expected := "- [ ] Buy groceries\n  - [ ] Whole milk 2L\n  - [x] Bread\n- [x] Clean kitchen"
	if string(printed) != expected {
		t.Fatalf("printed:\n%q\nexpected:\n%q", string(printed), expected)
	}
}

func TestPrintTaskFile_EmptyList(t *testing.T) {
	printed := PrintTaskFile(nil)
	if len(printed) != 0 {
		t.Fatalf("expected empty output, got %q", string(printed))
	}
}
