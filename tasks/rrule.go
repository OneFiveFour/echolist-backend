package tasks

import (
	"fmt"
	"time"

	"github.com/teambition/rrule-go"
)

// ComputeNextDueDate computes the next occurrence from an RRULE string after the given time.
func ComputeNextDueDate(rruleStr string, after time.Time) (time.Time, error) {
	r, err := rrule.StrToRRule(rruleStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid recurrence rule: %w", err)
	}
	r.DTStart(after)
	next := r.After(after, false)
	if next.IsZero() {
		return time.Time{}, fmt.Errorf("no next occurrence for recurrence rule %q after %s", rruleStr, after.Format("2006-01-02"))
	}
	return next, nil
}

// ValidateRRule checks if an RRULE string conforms to RFC 5545 syntax.
func ValidateRRule(rruleStr string) error {
	_, err := rrule.StrToRRule(rruleStr)
	if err != nil {
		return fmt.Errorf("invalid recurrence rule: %w", err)
	}
	return nil
}
