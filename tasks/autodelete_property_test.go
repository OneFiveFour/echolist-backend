package tasks

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// autoDeleteMainTaskGen generates a MainTask with random done/open state, optional recurrence,
// and 0-3 subtasks with random done states.
func autoDeleteMainTaskGen() *rapid.Generator[MainTask] {
	return rapid.Custom[MainTask](func(t *rapid.T) MainTask {
		desc := rapid.StringMatching(`[A-Za-z0-9 ]{1,40}`).Draw(t, "desc")
		done := rapid.Bool().Draw(t, "done")
		recurrence := rapid.SampledFrom([]string{"", "FREQ=DAILY", "FREQ=WEEKLY"}).Draw(t, "recurrence")
		dueDate := ""
		if recurrence != "" {
			dueDate = "2026-04-07"
		}
		numSubs := rapid.IntRange(0, 3).Draw(t, "numSubs")
		var subs []SubTask
		for i := 0; i < numSubs; i++ {
			subs = append(subs, SubTask{
				Description: rapid.StringMatching(`[A-Za-z0-9 ]{1,30}`).Draw(t, fmt.Sprintf("sub-%d", i)),
				IsDone:      rapid.Bool().Draw(t, fmt.Sprintf("sub-done-%d", i)),
			})
		}
		return MainTask{
			Description: desc,
			IsDone:      done,
			DueDate:     dueDate,
			Recurrence:  recurrence,
			SubTasks:    subs,
		}
	})
}

// autoDeleteMainTaskListGen generates a slice of 1-10 MainTasks.
func autoDeleteMainTaskListGen() *rapid.Generator[[]MainTask] {
	return rapid.SliceOfN(autoDeleteMainTaskGen(), 1, 10)
}

// Feature: tasklist-autodelete, Property 2: AutoDelete removes done non-recurring MainTasks
// Validates: Requirements 3.1, 6.2, 7.1
func TestProperty2_AutoDeleteRemovesDoneNonRecurring(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tasks := autoDeleteMainTaskListGen().Draw(rt, "tasks")
		result := filterAutoDeleted(tasks)

		// No done non-recurring MainTask should survive.
		for i, mt := range result {
			if mt.IsDone && mt.Recurrence == "" {
				rt.Fatalf("result[%d]: done non-recurring MainTask %q should have been removed", i, mt.Description)
			}
		}

		// The result should contain exactly the MainTasks that are either open or recurring.
		var expected int
		for _, mt := range tasks {
			if !(mt.IsDone && mt.Recurrence == "") {
				expected++
			}
		}
		if len(result) != expected {
			rt.Fatalf("expected %d surviving MainTasks, got %d", expected, len(result))
		}
	})
}

// Feature: tasklist-autodelete, Property 5: AutoDelete removes done SubTasks
// Validates: Requirements 4.1
func TestProperty5_AutoDeleteRemovesDoneSubTasks(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tasks := autoDeleteMainTaskListGen().Draw(rt, "tasks")
		result := filterAutoDeleted(tasks)

		// No done SubTask should survive on any surviving MainTask.
		for i, mt := range result {
			for j, st := range mt.SubTasks {
				if st.IsDone {
					rt.Fatalf("result[%d].SubTasks[%d]: done SubTask %q should have been removed", i, j, st.Description)
				}
			}
		}

		// For each surviving MainTask, all open SubTasks from the input should be retained.
		// filterAutoDeleted preserves order, so we walk input and result in lockstep.
		ri := 0
		for _, mt := range tasks {
			if mt.IsDone && mt.Recurrence == "" {
				continue // this MainTask was removed
			}
			if ri >= len(result) {
				rt.Fatal("fewer surviving MainTasks than expected")
			}
			rmt := result[ri]
			ri++

			// Count open subtasks in input.
			var expectedOpen int
			for _, st := range mt.SubTasks {
				if !st.IsDone {
					expectedOpen++
				}
			}
			if len(rmt.SubTasks) != expectedOpen {
				rt.Fatalf("MainTask %q: expected %d open SubTasks, got %d", mt.Description, expectedOpen, len(rmt.SubTasks))
			}
		}
	})
}
