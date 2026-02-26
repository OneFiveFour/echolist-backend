package tasks

import (
	"testing"
	"time"
)

func TestComputeNextDueDate_Weekly_Monday(t *testing.T) {
	// Wednesday 2026-01-07 → next Monday is 2026-01-12
	after := time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)
	next, err := ComputeNextDueDate("FREQ=WEEKLY;BYDAY=MO", after)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if next.Weekday() != time.Monday {
		t.Errorf("expected Monday, got %s", next.Weekday())
	}
	if !next.After(after) {
		t.Errorf("expected next (%s) to be after %s", next.Format("2006-01-02"), after.Format("2006-01-02"))
	}
}

func TestComputeNextDueDate_Daily(t *testing.T) {
	after := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	next, err := ComputeNextDueDate("FREQ=DAILY", after)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %s, got %s", expected.Format("2006-01-02"), next.Format("2006-01-02"))
	}
}

func TestComputeNextDueDate_Monthly(t *testing.T) {
	after := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	next, err := ComputeNextDueDate("FREQ=MONTHLY", after)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !next.After(after) {
		t.Errorf("expected next (%s) to be after %s", next.Format("2006-01-02"), after.Format("2006-01-02"))
	}
}

func TestComputeNextDueDate_Yearly(t *testing.T) {
	after := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	next, err := ComputeNextDueDate("FREQ=YEARLY", after)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("expected %s, got %s", expected.Format("2006-01-02"), next.Format("2006-01-02"))
	}
}

func TestComputeNextDueDate_WithInterval(t *testing.T) {
	after := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	next, err := ComputeNextDueDate("FREQ=WEEKLY;INTERVAL=2", after)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !next.After(after) {
		t.Errorf("expected next (%s) to be after %s", next.Format("2006-01-02"), after.Format("2006-01-02"))
	}
}

func TestComputeNextDueDate_InvalidRRule(t *testing.T) {
	_, err := ComputeNextDueDate("NOT_A_VALID_RRULE", time.Now())
	if err == nil {
		t.Fatal("expected error for invalid RRULE, got nil")
	}
}

func TestValidateRRule_Valid(t *testing.T) {
	valid := []string{
		"FREQ=DAILY",
		"FREQ=WEEKLY",
		"FREQ=WEEKLY;BYDAY=MO",
		"FREQ=WEEKLY;BYDAY=MO,WE,FR",
		"FREQ=MONTHLY",
		"FREQ=YEARLY",
		"FREQ=WEEKLY;INTERVAL=2",
		"FREQ=MONTHLY;INTERVAL=3",
	}
	for _, r := range valid {
		if err := ValidateRRule(r); err != nil {
			t.Errorf("ValidateRRule(%q) returned error: %v", r, err)
		}
	}
}

func TestValidateRRule_Invalid(t *testing.T) {
	invalid := []string{
		"NOT_A_RULE",
		"FREQ=",
		"",
		"FREQ=BOGUS",
		"garbage text",
	}
	for _, r := range invalid {
		if err := ValidateRRule(r); err == nil {
			t.Errorf("ValidateRRule(%q) expected error, got nil", r)
		}
	}
}
