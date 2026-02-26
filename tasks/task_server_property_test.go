package tasks

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	pb "echolist-backend/proto/gen/tasks/v1"
	"pgregory.net/rapid"
)

// validNameGen generates valid task list names (alphanumeric + hyphens/underscores, 1-30 chars).
func validNameGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]{0,29}`)
}

// simpleTaskGen generates a simple MainTask (no due date, no recurrence) for server-level tests.
func simpleTaskGen() *rapid.Generator[*pb.MainTask] {
	return rapid.Custom[*pb.MainTask](func(t *rapid.T) *pb.MainTask {
		desc := rapid.StringMatching(`[A-Za-z0-9 ]{1,40}`).Draw(t, "desc")
		done := rapid.Bool().Draw(t, "done")
		numSubs := rapid.IntRange(0, 3).Draw(t, "numSubs")
		var subs []*pb.Subtask
		for i := 0; i < numSubs; i++ {
			subs = append(subs, &pb.Subtask{
				Description: rapid.StringMatching(`[A-Za-z0-9 ]{1,30}`).Draw(t, fmt.Sprintf("sub-%d", i)),
				Done:        rapid.Bool().Draw(t, fmt.Sprintf("sub-done-%d", i)),
			})
		}
		return &pb.MainTask{Description: desc, Done: done, Subtasks: subs}
	})
}

// simpleTaskListGen generates a slice of 1-5 simple proto MainTasks.
func simpleTaskListGen() *rapid.Generator[[]*pb.MainTask] {
	return rapid.SliceOfN(simpleTaskGen(), 1, 5)
}

// invalidRRuleGen generates strings that are not valid RRULEs.
func invalidRRuleGen() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{
		"NOT_A_RULE",
		"FREQ=",
		"FREQ=BOGUS",
		"garbage text",
		"FREQ DAILY",
		"freq=daily",
		"INTERVAL=2",
	})
}

// traversalPathGen generates paths containing ".." or other traversal sequences.
func traversalPathGen() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{
		"../etc/passwd",
		"../../secret",
		"foo/../../bar",
		"../",
		"foo/../../../etc",
		"..%2f..%2f",
	})
}

// Feature: task-management, Property 5: Created task lists use tasks_ prefix
// **Validates: Requirements 3.1, 7.1**
func TestProperty5_CreatedTaskListsUseTasksPrefix(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		resp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name:  name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		expectedFile := "tasks_" + name + ".md"
		if _, err := os.Stat(filepath.Join(tmp, expectedFile)); os.IsNotExist(err) {
			rt.Fatalf("expected file %q on disk", expectedFile)
		}
		if resp.FilePath != expectedFile {
			rt.Fatalf("expected file_path %q, got %q", expectedFile, resp.FilePath)
		}
	})
}

// Feature: task-management, Property 6: Task list create-then-get round-trip
// **Validates: Requirements 3.2, 3.4**
func TestProperty6_TaskListCreateThenGetRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name:  name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		getResp, err := srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			FilePath: createResp.FilePath,
		})
		if err != nil {
			rt.Fatalf("GetTaskList failed: %v", err)
		}

		if getResp.Name != name {
			rt.Fatalf("name mismatch: expected %q, got %q", name, getResp.Name)
		}
		if len(getResp.Tasks) != len(tasks) {
			rt.Fatalf("task count mismatch: expected %d, got %d", len(tasks), len(getResp.Tasks))
		}
		for i, got := range getResp.Tasks {
			want := tasks[i]
			if got.Description != want.Description {
				rt.Fatalf("task %d description: expected %q, got %q", i, want.Description, got.Description)
			}
			if got.Done != want.Done {
				rt.Fatalf("task %d done: expected %v, got %v", i, want.Done, got.Done)
			}
			if len(got.Subtasks) != len(want.Subtasks) {
				rt.Fatalf("task %d subtask count: expected %d, got %d", i, len(want.Subtasks), len(got.Subtasks))
			}
			for j, gs := range got.Subtasks {
				ws := want.Subtasks[j]
				if gs.Description != ws.Description || gs.Done != ws.Done {
					rt.Fatalf("task %d subtask %d mismatch", i, j)
				}
			}
		}
	})
}

// Feature: task-management, Property 7: Duplicate task list name returns already-exists
// **Validates: Requirements 3.6**
func TestProperty7_DuplicateNameReturnsAlreadyExists(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name:  name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("first CreateTaskList failed: %v", err)
		}

		_, err = srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name:  name,
			Tasks: tasks,
		})
		if err == nil {
			rt.Fatal("expected AlreadyExists error, got nil")
		}
		if connect.CodeOf(err) != connect.CodeAlreadyExists {
			rt.Fatalf("expected AlreadyExists, got %v", connect.CodeOf(err))
		}
	})
}

// Feature: task-management, Property 8: Operations on non-existent paths return not-found
// **Validates: Requirements 3.7, 3.8**
func TestProperty8_NonExistentPathsReturnNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")
		fakePath := "tasks_" + name + ".md"

		_, err := srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{FilePath: fakePath})
		if err == nil {
			rt.Fatal("expected NotFound for GetTaskList, got nil")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			rt.Fatalf("GetTaskList: expected NotFound, got %v", connect.CodeOf(err))
		}

		_, err = srv.DeleteTaskList(context.Background(), &pb.DeleteTaskListRequest{FilePath: fakePath})
		if err == nil {
			rt.Fatal("expected NotFound for DeleteTaskList, got nil")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			rt.Fatalf("DeleteTaskList: expected NotFound, got %v", connect.CodeOf(err))
		}
	})
}

// Feature: task-management, Property 9: Mutual exclusion of due date and recurrence
// **Validates: Requirements 4.5**
func TestProperty9_MutualExclusionDueDateAndRecurrence(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")
		dueDate := validDueDateGen().Draw(rt, "due")
		rrule := validRRuleGen().Draw(rt, "rrule")

		_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name: name,
			Tasks: []*pb.MainTask{{
				Description: "test task",
				DueDate:     dueDate,
				Recurrence:  rrule,
			}},
		})
		if err == nil {
			rt.Fatal("expected InvalidArgument for both due_date and recurrence, got nil")
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			rt.Fatalf("expected InvalidArgument, got %v", connect.CodeOf(err))
		}
	})
}

// Feature: task-management, Property 10: Valid RRULE produces a computed due date
// **Validates: Requirements 4.3, 6.1, 6.2**
func TestProperty10_ValidRRuleProducesComputedDueDate(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")
		rrule := validRRuleGen().Draw(rt, "rrule")

		resp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name: name,
			Tasks: []*pb.MainTask{{
				Description: "recurring task",
				Recurrence:  rrule,
			}},
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		if len(resp.Tasks) != 1 {
			rt.Fatalf("expected 1 task, got %d", len(resp.Tasks))
		}
		task := resp.Tasks[0]
		if task.DueDate == "" {
			rt.Fatal("expected non-empty due_date for recurring task")
		}
		if task.Recurrence != rrule {
			rt.Fatalf("recurrence mismatch: expected %q, got %q", rrule, task.Recurrence)
		}
	})
}

// Feature: task-management, Property 11: Recurring task done-advance cycle
// **Validates: Requirements 6.3, 6.4**
func TestProperty11_RecurringTaskDoneAdvanceCycle(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")
		rrule := validRRuleGen().Draw(rt, "rrule")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name: name,
			Tasks: []*pb.MainTask{{
				Description: "recurring task",
				Recurrence:  rrule,
			}},
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		originalDueDate := createResp.Tasks[0].DueDate

		// Mark the task as done
		updateResp, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			FilePath: createResp.FilePath,
			Tasks: []*pb.MainTask{{
				Description: "recurring task",
				Done:        true,
				Recurrence:  rrule,
			}},
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList failed: %v", err)
		}

		updated := updateResp.Tasks[0]
		if updated.Done {
			rt.Fatal("recurring task should be reset to done=false after advance")
		}
		if updated.DueDate <= originalDueDate {
			rt.Fatalf("due date should advance: original %q, got %q", originalDueDate, updated.DueDate)
		}
	})
}

// Feature: task-management, Property 12: Invalid RRULE rejected
// **Validates: Requirements 6.5**
func TestProperty12_InvalidRRuleRejected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")
		badRule := invalidRRuleGen().Draw(rt, "badRule")

		_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name: name,
			Tasks: []*pb.MainTask{{
				Description: "bad recurring",
				Recurrence:  badRule,
			}},
		})
		if err == nil {
			rt.Fatalf("expected InvalidArgument for invalid RRULE %q, got nil", badRule)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			rt.Fatalf("expected InvalidArgument, got %v", connect.CodeOf(err))
		}
	})
}

// Feature: task-management, Property 13: Path traversal prevention
// **Validates: Requirements 1.3, 9.1, 9.2, 9.3**
func TestProperty13_PathTraversalPrevention(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		badPath := traversalPathGen().Draw(rt, "badPath")

		// CreateTaskList
		_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name: "test",
			Path: badPath,
			Tasks: []*pb.MainTask{{Description: "task"}},
		})
		if err == nil {
			rt.Fatalf("CreateTaskList should reject traversal path %q", badPath)
		}

		// GetTaskList
		_, err = srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			FilePath: badPath + "/tasks_test.md",
		})
		if err == nil {
			rt.Fatalf("GetTaskList should reject traversal path %q", badPath)
		}

		// ListTaskLists
		_, err = srv.ListTaskLists(context.Background(), &pb.ListTaskListsRequest{
			Path: badPath,
		})
		if err == nil {
			rt.Fatalf("ListTaskLists should reject traversal path %q", badPath)
		}

		// UpdateTaskList
		_, err = srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			FilePath: badPath + "/tasks_test.md",
			Tasks:    []*pb.MainTask{{Description: "task"}},
		})
		if err == nil {
			rt.Fatalf("UpdateTaskList should reject traversal path %q", badPath)
		}

		// DeleteTaskList
		_, err = srv.DeleteTaskList(context.Background(), &pb.DeleteTaskListRequest{
			FilePath: badPath + "/tasks_test.md",
		})
		if err == nil {
			rt.Fatalf("DeleteTaskList should reject traversal path %q", badPath)
		}
	})
}

// Feature: task-management, Property 3: ListTaskLists excludes non-task files
// **Validates: Requirements 3.3**
func TestProperty3_ListTaskListsExcludesNonTaskFiles(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)

		usedNames := make(map[string]bool)

		// Create tasks_*.md files
		numTasks := rapid.IntRange(0, 5).Draw(rt, "numTasks")
		taskNames := make(map[string]bool)
		for i := 0; i < numTasks; i++ {
			name := validNameGen().Draw(rt, fmt.Sprintf("taskName-%d", i))
			fname := "tasks_" + name + ".md"
			if usedNames[fname] {
				continue
			}
			usedNames[fname] = true
			taskNames[name] = true
			os.WriteFile(filepath.Join(tmp, fname), []byte("- [ ] task"), 0644)
		}

		// Create note_*.md files
		numNotes := rapid.IntRange(0, 3).Draw(rt, "numNotes")
		for i := 0; i < numNotes; i++ {
			name := validNameGen().Draw(rt, fmt.Sprintf("noteName-%d", i))
			fname := "note_" + name + ".md"
			if usedNames[fname] {
				continue
			}
			usedNames[fname] = true
			os.WriteFile(filepath.Join(tmp, fname), []byte("note"), 0644)
		}

		// Create other files
		numOther := rapid.IntRange(0, 3).Draw(rt, "numOther")
		for i := 0; i < numOther; i++ {
			name := validNameGen().Draw(rt, fmt.Sprintf("otherName-%d", i))
			fname := name + ".txt"
			if usedNames[fname] {
				continue
			}
			usedNames[fname] = true
			os.WriteFile(filepath.Join(tmp, fname), []byte("other"), 0644)
		}

		// Create subdirectories
		numDirs := rapid.IntRange(0, 3).Draw(rt, "numDirs")
		dirNames := make(map[string]bool)
		for i := 0; i < numDirs; i++ {
			name := validNameGen().Draw(rt, fmt.Sprintf("dirName-%d", i))
			if usedNames[name] {
				continue
			}
			usedNames[name] = true
			dirNames[name] = true
			os.MkdirAll(filepath.Join(tmp, name), 0755)
		}

		resp, err := srv.ListTaskLists(context.Background(), &pb.ListTaskListsRequest{})
		if err != nil {
			rt.Fatalf("ListTaskLists failed: %v", err)
		}

		if len(resp.TaskLists) != len(taskNames) {
			rt.Fatalf("expected %d task lists, got %d", len(taskNames), len(resp.TaskLists))
		}
		for _, tl := range resp.TaskLists {
			if !taskNames[tl.Name] {
				rt.Fatalf("unexpected task list name %q", tl.Name)
			}
		}

		expectedEntries := len(taskNames) + len(dirNames)
		if len(resp.Entries) != expectedEntries {
			rt.Fatalf("expected %d entries, got %d: %v", expectedEntries, len(resp.Entries), resp.Entries)
		}
	})
}

// Feature: task-management, Property 15: Auto-create folders on task list creation
// **Validates: Requirements 8.2**
func TestProperty15_AutoCreateFoldersOnCreation(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")

		// Generate a nested path that doesn't exist yet
		seg1 := validNameGen().Draw(rt, "seg1")
		seg2 := validNameGen().Draw(rt, "seg2")
		nestedPath := filepath.Join(seg1, seg2)

		resp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name:  name,
			Path:  nestedPath,
			Tasks: []*pb.MainTask{{Description: "task in nested dir"}},
		})
		if err != nil {
			rt.Fatalf("CreateTaskList with nested path failed: %v", err)
		}

		// Verify directories were created
		dirPath := filepath.Join(tmp, seg1, seg2)
		info, err := os.Stat(dirPath)
		if err != nil {
			rt.Fatalf("expected directory %q to exist: %v", dirPath, err)
		}
		if !info.IsDir() {
			rt.Fatalf("expected %q to be a directory", dirPath)
		}

		// Verify file exists
		expectedFile := filepath.Join(nestedPath, "tasks_"+name+".md")
		if resp.FilePath != expectedFile {
			rt.Fatalf("expected file_path %q, got %q", expectedFile, resp.FilePath)
		}
	})
}

// Feature: task-management, Property 16: Delete removes task list from disk
// **Validates: Requirements 3.5**
func TestProperty16_DeleteRemovesTaskListFromDisk(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp)
		name := validNameGen().Draw(rt, "name")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Name:  name,
			Tasks: []*pb.MainTask{{Description: "to be deleted"}},
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		_, err = srv.DeleteTaskList(context.Background(), &pb.DeleteTaskListRequest{
			FilePath: createResp.FilePath,
		})
		if err != nil {
			rt.Fatalf("DeleteTaskList failed: %v", err)
		}

		// File must not exist
		absPath := filepath.Join(tmp, createResp.FilePath)
		if _, err := os.Stat(absPath); !os.IsNotExist(err) {
			rt.Fatalf("expected file %q to be deleted", absPath)
		}

		// GetTaskList must return NotFound
		_, err = srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			FilePath: createResp.FilePath,
		})
		if err == nil {
			rt.Fatal("expected NotFound after delete, got nil")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			rt.Fatalf("expected NotFound, got %v", connect.CodeOf(err))
		}
	})
}
