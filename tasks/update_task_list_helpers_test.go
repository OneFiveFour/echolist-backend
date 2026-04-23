package tasks

import (
	"testing"
)

// ---------------------------------------------------------------------------
// advanceRecurringTasks
// ---------------------------------------------------------------------------

func TestAdvanceRecurringTasks_SkipsNonRecurring(t *testing.T) {
	tasks := []MainTask{
		{Description: "plain task", IsDone: true},
		{Description: "open recurring", Recurrence: "FREQ=DAILY", IsDone: false, DueDate: "2026-04-01"},
	}
	if err := advanceRecurringTasks(tasks, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Plain done task stays done
	if !tasks[0].IsDone {
		t.Fatal("non-recurring task should remain done")
	}
	// Open recurring task stays open with original due date
	if tasks[1].IsDone {
		t.Fatal("open recurring task should remain not-done")
	}
	if tasks[1].DueDate != "2026-04-01" {
		t.Fatalf("open recurring task due date should be unchanged, got %q", tasks[1].DueDate)
	}
}

func TestAdvanceRecurringTasks_AdvancesDoneRecurring(t *testing.T) {
	tasks := []MainTask{
		{Description: "weekly", Recurrence: "FREQ=WEEKLY", IsDone: true, DueDate: "2026-03-01"},
	}
	existing := []MainTask{
		{Description: "weekly", Recurrence: "FREQ=WEEKLY", DueDate: "2026-03-01"},
	}

	if err := advanceRecurringTasks(tasks, existing); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tasks[0].IsDone {
		t.Fatal("recurring task should be reset to done=false")
	}
	if tasks[0].DueDate <= "2026-03-01" {
		t.Fatalf("due date should advance past 2026-03-01, got %q", tasks[0].DueDate)
	}
}

func TestAdvanceRecurringTasks_UsesOwnDueDateWhenNoExisting(t *testing.T) {
	tasks := []MainTask{
		{Description: "daily", Recurrence: "FREQ=DAILY", IsDone: true, DueDate: "2026-06-15"},
	}

	if err := advanceRecurringTasks(tasks, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tasks[0].IsDone {
		t.Fatal("should be reset to done=false")
	}
	if tasks[0].DueDate != "2026-06-16" {
		t.Fatalf("expected 2026-06-16, got %q", tasks[0].DueDate)
	}
}

func TestAdvanceRecurringTasks_InvalidRRuleReturnsError(t *testing.T) {
	tasks := []MainTask{
		{Description: "bad", Recurrence: "NOT_VALID", IsDone: true},
	}
	err := advanceRecurringTasks(tasks, nil)
	if err == nil {
		t.Fatal("expected error for invalid RRULE, got nil")
	}
}

func TestAdvanceRecurringTasks_EmptySlice(t *testing.T) {
	if err := advanceRecurringTasks(nil, nil); err != nil {
		t.Fatalf("unexpected error on empty input: %v", err)
	}
}
