package tasks

import (
	"context"
	"fmt"
	"testing"

	pb "echolist-backend/proto/gen/tasks/v1"
	"pgregory.net/rapid"
)

// Feature: tasklist-message-unification, Property 1: Create-then-Get round trip through TaskList message
// **Validates: Requirements 2.2, 3.2**
func TestProperty_CreateGetRoundTripTaskListMessage(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		// CreateTaskListResponse.TaskList must be non-nil
		if createResp.TaskList == nil {
			rt.Fatal("CreateTaskListResponse.TaskList is nil")
		}

		getResp, err := srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			Id: createResp.TaskList.Id,
		})
		if err != nil {
			rt.Fatalf("GetTaskList failed: %v", err)
		}

		// GetTaskListResponse.TaskList must be non-nil
		if getResp.TaskList == nil {
			rt.Fatal("GetTaskListResponse.TaskList is nil")
		}

		// Both must share the same parent_dir
		if createResp.TaskList.ParentDir != getResp.TaskList.ParentDir {
			rt.Fatalf("parent_dir mismatch: create=%q get=%q", createResp.TaskList.ParentDir, getResp.TaskList.ParentDir)
		}

		// Both must share the same name
		if createResp.TaskList.Title != getResp.TaskList.Title {
			rt.Fatalf("name mismatch: create=%q get=%q", createResp.TaskList.Title, getResp.TaskList.Title)
		}
		if getResp.TaskList.Title != name {
			rt.Fatalf("name mismatch: expected %q, got %q", name, getResp.TaskList.Title)
		}

		// Both must contain the same tasks
		if len(createResp.TaskList.Tasks) != len(tasks) {
			rt.Fatalf("create task count mismatch: expected %d, got %d", len(tasks), len(createResp.TaskList.Tasks))
		}
		if len(getResp.TaskList.Tasks) != len(tasks) {
			rt.Fatalf("get task count mismatch: expected %d, got %d", len(tasks), len(getResp.TaskList.Tasks))
		}
		for i, want := range tasks {
			cGot := createResp.TaskList.Tasks[i]
			gGot := getResp.TaskList.Tasks[i]
			if cGot.Description != want.Description {
				rt.Fatalf("create task %d description: expected %q, got %q", i, want.Description, cGot.Description)
			}
			if gGot.Description != want.Description {
				rt.Fatalf("get task %d description: expected %q, got %q", i, want.Description, gGot.Description)
			}
			if cGot.IsDone != want.IsDone {
				rt.Fatalf("create task %d done: expected %v, got %v", i, want.IsDone, cGot.IsDone)
			}
			if gGot.IsDone != want.IsDone {
				rt.Fatalf("get task %d done: expected %v, got %v", i, want.IsDone, gGot.IsDone)
			}
		}

		// Both must have positive updated_at
		if createResp.TaskList.UpdatedAt <= 0 {
			rt.Fatalf("create updated_at should be positive, got %d", createResp.TaskList.UpdatedAt)
		}
		if getResp.TaskList.UpdatedAt <= 0 {
			rt.Fatalf("get updated_at should be positive, got %d", getResp.TaskList.UpdatedAt)
		}
	})
}

// Feature: tasklist-message-unification, Property 2: Update returns correct TaskList message
// **Validates: Requirements 4.2**
func TestProperty_UpdateReturnsTaskListMessage(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, nopLogger())
		name := validNameGen().Draw(rt, "name")
		initialTasks := simpleTaskListGen().Draw(rt, "initialTasks")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: initialTasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		// Generate new tasks for the update
		updatedTasks := simpleTaskListGen().Draw(rt, "updatedTasks")

		updateResp, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			Id:    createResp.TaskList.Id,
			Title: name,
			Tasks: updatedTasks,
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList failed: %v", err)
		}

		// UpdateTaskListResponse.TaskList must be non-nil
		if updateResp.TaskList == nil {
			rt.Fatal("UpdateTaskListResponse.TaskList is nil")
		}

		// parent_dir must match the original
		if updateResp.TaskList.ParentDir != createResp.TaskList.ParentDir {
			rt.Fatalf("parent_dir mismatch: expected %q, got %q", createResp.TaskList.ParentDir, updateResp.TaskList.ParentDir)
		}

		// name must match the original
		if updateResp.TaskList.Title != name {
			rt.Fatalf("name mismatch: expected %q, got %q", name, updateResp.TaskList.Title)
		}

		// tasks must reflect the update
		if len(updateResp.TaskList.Tasks) != len(updatedTasks) {
			rt.Fatalf("task count mismatch: expected %d, got %d", len(updatedTasks), len(updateResp.TaskList.Tasks))
		}
		for i, want := range updatedTasks {
			got := updateResp.TaskList.Tasks[i]
			if got.Description != want.Description {
				rt.Fatalf("task %d description: expected %q, got %q", i, want.Description, got.Description)
			}
			if got.IsDone != want.IsDone {
				rt.Fatalf("task %d done: expected %v, got %v", i, want.IsDone, got.IsDone)
			}
			if len(got.SubTasks) != len(want.SubTasks) {
				rt.Fatalf("task %d subtask count: expected %d, got %d", i, len(want.SubTasks), len(got.SubTasks))
			}
		}

		// updated_at must be positive
		if updateResp.TaskList.UpdatedAt <= 0 {
			rt.Fatalf("updated_at should be positive, got %d", updateResp.TaskList.UpdatedAt)
		}
	})
}

// Feature: tasklist-message-unification, Property 3: ListTaskLists returns full TaskList messages with tasks and entries
// **Validates: Requirements 5.2, 5.4**
func TestProperty_ListReturnsFullTaskListMessages(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, nopLogger())

		// Create 1-4 task lists with unique names
		numLists := rapid.IntRange(1, 4).Draw(rt, "numLists")
		usedNames := make(map[string]bool)
		type created struct {
			name  string
			tasks []*pb.MainTask
		}
		var createdLists []created

		for i := 0; i < numLists; i++ {
			name := validNameGen().Draw(rt, fmt.Sprintf("name-%d", i))
			if usedNames[name] {
				continue
			}
			usedNames[name] = true
			tasks := simpleTaskListGen().Draw(rt, fmt.Sprintf("tasks-%d", i))

			_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
				Title: name,
				Tasks: tasks,
			})
			if err != nil {
				rt.Fatalf("CreateTaskList %q failed: %v", name, err)
			}
			createdLists = append(createdLists, created{name: name, tasks: tasks})
		}

		listResp, err := srv.ListTaskLists(context.Background(), &pb.ListTaskListsRequest{})
		if err != nil {
			rt.Fatalf("ListTaskLists failed: %v", err)
		}

		// Number of task_lists must match created count
		if len(listResp.TaskLists) != len(createdLists) {
			rt.Fatalf("expected %d task lists, got %d", len(createdLists), len(listResp.TaskLists))
		}

		// Build a lookup by name for verification
		tlByName := make(map[string]*pb.TaskList)
		for _, tl := range listResp.TaskLists {
			tlByName[tl.Title] = tl
		}

		for _, c := range createdLists {
			tl, ok := tlByName[c.name]
			if !ok {
				rt.Fatalf("task list %q not found in ListTaskLists response", c.name)
			}

			// tasks field must be non-empty and match the originally created tasks
			if len(tl.Tasks) == 0 {
				rt.Fatalf("task list %q has empty tasks", c.name)
			}
			if len(tl.Tasks) != len(c.tasks) {
				rt.Fatalf("task list %q task count: expected %d, got %d", c.name, len(c.tasks), len(tl.Tasks))
			}
			for i, want := range c.tasks {
				got := tl.Tasks[i]
				if got.Description != want.Description {
					rt.Fatalf("task list %q task %d description: expected %q, got %q", c.name, i, want.Description, got.Description)
				}
			}
		}

		// Verify parent_dir is set correctly on each TaskList
		for _, c := range createdLists {
			tl := tlByName[c.name]
			if tl.ParentDir != "" {
				rt.Fatalf("task list %q parent_dir: expected %q, got %q", c.name, "", tl.ParentDir)
			}
		}
	})
}
