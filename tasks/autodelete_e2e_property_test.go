package tasks

import (
	"context"
	"fmt"
	"testing"

	pb "echolist-backend/proto/gen/tasks/v1"
	"pgregory.net/rapid"
)

// Feature: tasklist-autodelete, Property 1: AutoDelete flag round-trip
// For any boolean value v and any valid task list, creating (or updating) the task list
// with is_auto_delete = v and then reading it back via GetTaskList (or ListTaskLists,
// or the UpdateTaskList response) should return is_auto_delete == v.
// Validates: Requirements 1.4, 2.1, 2.2, 2.3, 2.5, 7.3
func TestProperty1_AutoDeleteFlagRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")
		autoDelete := rapid.Bool().Draw(rt, "autoDelete")

		// Create with the flag
		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title:        name,
			Tasks:        tasks,
			IsAutoDelete: autoDelete,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}
		if createResp.TaskList.IsAutoDelete != autoDelete {
			rt.Fatalf("CreateTaskList response: expected is_auto_delete=%v, got %v", autoDelete, createResp.TaskList.IsAutoDelete)
		}

		// GetTaskList round-trip
		getResp, err := srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			Id: createResp.TaskList.Id,
		})
		if err != nil {
			rt.Fatalf("GetTaskList failed: %v", err)
		}
		if getResp.TaskList.IsAutoDelete != autoDelete {
			rt.Fatalf("GetTaskList: expected is_auto_delete=%v, got %v", autoDelete, getResp.TaskList.IsAutoDelete)
		}

		// ListTaskLists round-trip
		listResp, err := srv.ListTaskLists(context.Background(), &pb.ListTaskListsRequest{})
		if err != nil {
			rt.Fatalf("ListTaskLists failed: %v", err)
		}
		var found bool
		for _, tl := range listResp.TaskLists {
			if tl.Id == createResp.TaskList.Id {
				found = true
				if tl.IsAutoDelete != autoDelete {
					rt.Fatalf("ListTaskLists: expected is_auto_delete=%v, got %v", autoDelete, tl.IsAutoDelete)
				}
			}
		}
		if !found {
			rt.Fatal("ListTaskLists did not include the created task list")
		}

		// UpdateTaskList round-trip: flip the flag
		flipped := !autoDelete
		updateResp, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			Id:           createResp.TaskList.Id,
			Title:        name,
			Tasks:        tasks,
			IsAutoDelete: flipped,
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList failed: %v", err)
		}
		if updateResp.TaskList.IsAutoDelete != flipped {
			rt.Fatalf("UpdateTaskList response: expected is_auto_delete=%v, got %v", flipped, updateResp.TaskList.IsAutoDelete)
		}

		// Verify the flipped value persisted via GetTaskList
		getResp2, err := srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			Id: createResp.TaskList.Id,
		})
		if err != nil {
			rt.Fatalf("GetTaskList after update failed: %v", err)
		}
		if getResp2.TaskList.IsAutoDelete != flipped {
			rt.Fatalf("GetTaskList after update: expected is_auto_delete=%v, got %v", flipped, getResp2.TaskList.IsAutoDelete)
		}
	})
}

// Feature: tasklist-autodelete, Property 3: AutoDelete disabled retains all tasks
// For any task list with AutoDelete disabled and for any set of MainTasks and SubTasks
// (regardless of their done status), after an UpdateTaskList call, the response and
// persisted task list should contain every MainTask and every SubTask that was sent
// in the request, with their done status preserved.
// Validates: Requirements 3.2, 4.2, 7.2
func TestProperty3_AutoDeleteDisabledRetainsAll(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, nopLogger())
		name := validNameGen().Draw(rt, "name")

		// Create with AutoDelete OFF
		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title:        name,
			Tasks:        []*pb.MainTask{{Description: "seed"}},
			IsAutoDelete: false,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		// Generate random tasks with mix of done states
		tasks := simpleTaskListGen().Draw(rt, "tasks")
		// Mark some as done to exercise the retention path
		for i := range tasks {
			tasks[i].IsDone = rapid.Bool().Draw(rt, fmt.Sprintf("done-%d", i))
		}

		updateResp, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			Id:           createResp.TaskList.Id,
			Title:        name,
			Tasks:        tasks,
			IsAutoDelete: false,
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList failed: %v", err)
		}

		got := updateResp.TaskList.Tasks
		if len(got) != len(tasks) {
			rt.Fatalf("expected %d MainTasks, got %d", len(tasks), len(got))
		}
		for i, g := range got {
			w := tasks[i]
			if g.Description != w.Description {
				rt.Fatalf("task %d description: expected %q, got %q", i, w.Description, g.Description)
			}
			if g.IsDone != w.IsDone {
				rt.Fatalf("task %d done: expected %v, got %v", i, w.IsDone, g.IsDone)
			}
			if len(g.SubTasks) != len(w.SubTasks) {
				rt.Fatalf("task %d subtask count: expected %d, got %d", i, len(w.SubTasks), len(g.SubTasks))
			}
			for j, gs := range g.SubTasks {
				ws := w.SubTasks[j]
				if gs.Description != ws.Description || gs.IsDone != ws.IsDone {
					rt.Fatalf("task %d subtask %d mismatch", i, j)
				}
			}
		}
	})
}

// Feature: tasklist-autodelete, Property 4: AutoDelete advances recurring tasks instead of deleting
// For any task list with AutoDelete enabled and for any recurring MainTask marked as
// done = true, after an UpdateTaskList call, the MainTask should still be present in
// the result with done = false and a due date strictly after the previous due date.
// Validates: Requirements 3.3, 6.1
func TestProperty4_AutoDeleteAdvancesRecurring(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, nopLogger())
		name := validNameGen().Draw(rt, "name")
		rrule := validRRuleGen().Draw(rt, "rrule")

		// Create with AutoDelete ON and a recurring task
		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: []*pb.MainTask{{
				Description: "recurring task",
				Recurrence:  rrule,
			}},
			IsAutoDelete: true,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		originalDueDate := createResp.TaskList.Tasks[0].DueDate

		// Mark the recurring task as done with AutoDelete ON
		updateResp, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			Id:    createResp.TaskList.Id,
			Title: name,
			Tasks: []*pb.MainTask{{
				Description: "recurring task",
				IsDone:      true,
				Recurrence:  rrule,
			}},
			IsAutoDelete: true,
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList failed: %v", err)
		}

		// The recurring task must still be present
		if len(updateResp.TaskList.Tasks) != 1 {
			rt.Fatalf("expected 1 task (recurring should survive), got %d", len(updateResp.TaskList.Tasks))
		}

		updated := updateResp.TaskList.Tasks[0]
		if updated.IsDone {
			rt.Fatal("recurring task should be reset to done=false after advance")
		}
		if updated.DueDate <= originalDueDate {
			rt.Fatalf("due date should advance: original %q, got %q", originalDueDate, updated.DueDate)
		}
		if updated.Recurrence != rrule {
			rt.Fatalf("recurrence should be preserved: expected %q, got %q", rrule, updated.Recurrence)
		}
	})
}

// Feature: tasklist-autodelete, Property 6: Manual deletion cascades regardless of AutoDelete
// For any task list (regardless of AutoDelete mode) and for any MainTask that exists
// in the persisted list but is omitted from the UpdateTaskList request, that MainTask
// and all of its SubTasks should be absent from the response and persisted task list.
// The result of omitting a task should be identical whether AutoDelete is enabled or disabled.
// Validates: Requirements 5.1, 5.2
func TestProperty6_ManualDeletionCascadesRegardlessOfAutoDelete(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, nopLogger())
		name := validNameGen().Draw(rt, "name")

		// Generate 2-5 tasks with subtasks (none recurring to avoid recurrence side effects)
		numTasks := rapid.IntRange(2, 5).Draw(rt, "numTasks")
		var originalTasks []*pb.MainTask
		for i := 0; i < numTasks; i++ {
			numSubs := rapid.IntRange(0, 3).Draw(rt, fmt.Sprintf("numSubs-%d", i))
			var subs []*pb.SubTask
			for j := 0; j < numSubs; j++ {
				subs = append(subs, &pb.SubTask{
					Description: fmt.Sprintf("sub-%d-%d", i, j),
					IsDone:      rapid.Bool().Draw(rt, fmt.Sprintf("sub-done-%d-%d", i, j)),
				})
			}
			originalTasks = append(originalTasks, &pb.MainTask{
				Description: fmt.Sprintf("task-%d", i),
				IsDone:      rapid.Bool().Draw(rt, fmt.Sprintf("done-%d", i)),
				SubTasks:    subs,
			})
		}

		// Create two task lists: one with AutoDelete ON, one OFF
		createOn, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title:        name + "On",
			Tasks:        originalTasks,
			IsAutoDelete: true,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList (on) failed: %v", err)
		}
		createOff, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title:        name + "Off",
			Tasks:        originalTasks,
			IsAutoDelete: false,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList (off) failed: %v", err)
		}

		// Omit the first task from the update (manual deletion)
		keptTasks := originalTasks[1:]

		updateOn, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			Id:           createOn.TaskList.Id,
			Title:        name + "On",
			Tasks:        keptTasks,
			IsAutoDelete: true,
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList (on) failed: %v", err)
		}
		updateOff, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			Id:           createOff.TaskList.Id,
			Title:        name + "Off",
			Tasks:        keptTasks,
			IsAutoDelete: false,
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList (off) failed: %v", err)
		}

		// The omitted task must be absent from both responses
		for _, resp := range []*pb.UpdateTaskListResponse{updateOn, updateOff} {
			for _, t := range resp.TaskList.Tasks {
				if t.Description == originalTasks[0].Description {
					rt.Fatalf("manually deleted task %q should be absent", originalTasks[0].Description)
				}
			}
		}

		// Both responses should have the same number of tasks remaining
		// (AutoDelete may remove additional done tasks from the ON list, but the
		// manually deleted task should be gone from both)
		// Verify the omitted task is gone from both by checking descriptions
		onDescs := make(map[string]bool)
		for _, t := range updateOn.TaskList.Tasks {
			onDescs[t.Description] = true
		}
		offDescs := make(map[string]bool)
		for _, t := range updateOff.TaskList.Tasks {
			offDescs[t.Description] = true
		}

		// The manually deleted task must be absent from both
		if onDescs[originalTasks[0].Description] {
			rt.Fatal("manually deleted task present in AutoDelete ON response")
		}
		if offDescs[originalTasks[0].Description] {
			rt.Fatal("manually deleted task present in AutoDelete OFF response")
		}

		// All kept tasks that survive AutoDelete filtering in the ON list
		// must also be present in the OFF list (OFF retains everything)
		for desc := range onDescs {
			if !offDescs[desc] {
				rt.Fatalf("task %q present in ON response but missing from OFF response", desc)
			}
		}
	})
}
