package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"echolist-backend/common"
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
		var subs []*pb.SubTask
		for i := 0; i < numSubs; i++ {
			subs = append(subs, &pb.SubTask{
				Description: rapid.StringMatching(`[A-Za-z0-9 ]{1,30}`).Draw(t, fmt.Sprintf("sub-%d", i)),
				IsDone:      rapid.Bool().Draw(t, fmt.Sprintf("sub-done-%d", i)),
			})
		}
		return &pb.MainTask{Description: desc, IsDone: done, SubTasks: subs}
	})
}

// simpleTaskListGen generates a slice of 1-5 simple proto MainTasks.
func simpleTaskListGen() *rapid.Generator[[]*pb.MainTask] {
	return rapid.SliceOfN(simpleTaskGen(), 1, 5)
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
	})
}
// uuidV4Gen generates valid UUIDv4 strings for testing NotFound scenarios.
func uuidV4Gen() *rapid.Generator[string] {
	return rapid.Custom(func(rt *rapid.T) string {
		a := rapid.StringMatching(`[0-9a-f]{8}`).Draw(rt, "a")
		b := rapid.StringMatching(`[0-9a-f]{4}`).Draw(rt, "b")
		c := rapid.StringMatching(`[0-9a-f]{3}`).Draw(rt, "c")
		d := rapid.StringMatching(`[89ab][0-9a-f]{3}`).Draw(rt, "d")
		e := rapid.StringMatching(`[0-9a-f]{12}`).Draw(rt, "e")
		return fmt.Sprintf("%s-%s-4%s-%s-%s", a, b, c, d, e)
	})
}

// assertCodeNotFound checks that the error is a connect.Error with CodeNotFound.
func assertCodeNotFound(rt *rapid.T, err error, handler, id string) {
	rt.Helper()
	if err == nil {
		rt.Fatalf("%s: expected error for non-existent id %q, got nil", handler, id)
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		rt.Fatalf("%s: expected connect.Error for id %q, got %T: %v", handler, id, err, err)
	}
	if connErr.Code() != connect.CodeNotFound {
		rt.Fatalf("%s: expected CodeNotFound for id %q, got %v", handler, id, connErr.Code())
	}
}

func assertCodeInvalidArgument(rt *rapid.T, err error, handler, id string) {
	rt.Helper()
	if err == nil {
		rt.Fatalf("%s: expected error for invalid id %q, got nil", handler, id)
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) {
		rt.Fatalf("%s: expected connect.Error for id %q, got %T: %v", handler, id, err, err)
	}
	if connErr.Code() != connect.CodeInvalidArgument {
		rt.Fatalf("%s: expected CodeInvalidArgument for id %q, got %v", handler, id, connErr.Code())
	}
}

// Feature: task-management, Property 5: Created task lists use tasks_ prefix
// **Validates: Requirements 3.1, 7.1**
func TestProperty5_CreatedTaskListsUseTasksPrefix(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		resp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		expectedFile := "tasks_" + name + ".md"
		if _, err := os.Stat(filepath.Join(tmp, expectedFile)); os.IsNotExist(err) {
			rt.Fatalf("expected file %q on disk", expectedFile)
		}
		if resp.TaskList.ParentDir != "" {
			rt.Fatalf("expected parent_dir %q, got %q", "", resp.TaskList.ParentDir)
		}
	})
}

// Feature: task-management, Property 6: Task list create-then-get round-trip
// **Validates: Requirements 3.2, 3.4**
func TestProperty6_TaskListCreateThenGetRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		getResp, err := srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			Id: createResp.TaskList.Id,
		})
		if err != nil {
			rt.Fatalf("GetTaskList failed: %v", err)
		}

		if getResp.TaskList.Title != name {
			rt.Fatalf("name mismatch: expected %q, got %q", name, getResp.TaskList.Title)
		}
		if len(getResp.TaskList.Tasks) != len(tasks) {
			rt.Fatalf("task count mismatch: expected %d, got %d", len(tasks), len(getResp.TaskList.Tasks))
		}
		for i, got := range getResp.TaskList.Tasks {
			want := tasks[i]
			if got.Description != want.Description {
				rt.Fatalf("task %d description: expected %q, got %q", i, want.Description, got.Description)
			}
			if got.IsDone != want.IsDone {
				rt.Fatalf("task %d done: expected %v, got %v", i, want.IsDone, got.IsDone)
			}
			if len(got.SubTasks) != len(want.SubTasks) {
				rt.Fatalf("task %d subtask count: expected %d, got %d", i, len(want.SubTasks), len(got.SubTasks))
			}
			for j, gs := range got.SubTasks {
				ws := want.SubTasks[j]
				if gs.Description != ws.Description || gs.IsDone != ws.IsDone {
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
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("first CreateTaskList failed: %v", err)
		}

		_, err = srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
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

// Feature: task-management, Property 8: Operations on non-existent ids return not-found
// **Validates: Requirements 3.7, 3.8**
func TestProperty8_NonExistentPathsReturnNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		_ = name // keep the draw for backward compat with rapid seed files
		fakeId := "00000000-0000-4000-8000-000000000000"

		_, err := srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{Id: fakeId})
		if err == nil {
			rt.Fatal("expected NotFound for GetTaskList, got nil")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			rt.Fatalf("GetTaskList: expected NotFound, got %v", connect.CodeOf(err))
		}

		_, err = srv.DeleteTaskList(context.Background(), &pb.DeleteTaskListRequest{Id: fakeId})
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
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		dueDate := validDueDateGen().Draw(rt, "due")
		rrule := validRRuleGen().Draw(rt, "rrule")

		_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
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
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		rrule := validRRuleGen().Draw(rt, "rrule")

		resp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: []*pb.MainTask{{
				Description: "recurring task",
				Recurrence:  rrule,
			}},
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		if len(resp.TaskList.Tasks) != 1 {
			rt.Fatalf("expected 1 task, got %d", len(resp.TaskList.Tasks))
		}
		task := resp.TaskList.Tasks[0]
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
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		rrule := validRRuleGen().Draw(rt, "rrule")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: []*pb.MainTask{{
				Description: "recurring task",
				Recurrence:  rrule,
			}},
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		originalDueDate := createResp.TaskList.Tasks[0].DueDate

		// Mark the task as done
		updateResp, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			Id:    createResp.TaskList.Id,
			Title: name,
			Tasks: []*pb.MainTask{{
				Description: "recurring task",
				IsDone:      true,
				Recurrence:  rrule,
			}},
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList failed: %v", err)
		}

		updated := updateResp.TaskList.Tasks[0]
		if updated.IsDone {
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
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		badRule := invalidRRuleGen().Draw(rt, "badRule")

		_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
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
// Note: Get/Update/Delete RPCs now take a UUID id, not a file path, so path
// traversal is only relevant for CreateTaskList (parent_dir) and ListTaskLists (parent_dir).
func TestProperty13_PathTraversalPrevention(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		badPath := traversalPathGen().Draw(rt, "badPath")

		// CreateTaskList with traversal parent_dir
		_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title:     "test",
			ParentDir: badPath,
			Tasks:     []*pb.MainTask{{Description: "task"}},
		})
		if err == nil {
			rt.Fatalf("CreateTaskList should reject traversal path %q", badPath)
		}

		// ListTaskLists with traversal parent_dir
		_, err = srv.ListTaskLists(context.Background(), &pb.ListTaskListsRequest{
			ParentDir: badPath,
		})
		if err == nil {
			rt.Fatalf("ListTaskLists should reject traversal path %q", badPath)
		}
	})
}

// Feature: task-management, Property 3: ListTaskLists excludes non-task files
// **Validates: Requirements 3.3**
func TestProperty3_ListTaskListsExcludesNonTaskFiles(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())

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
			if !taskNames[tl.Title] {
				rt.Fatalf("unexpected task list name %q", tl.Title)
			}
		}
	})
}

// Feature: task-management, Property 15: Auto-create folders on task list creation
// **Validates: Requirements 8.2**
// Feature: task-management, Property 15: Non-existent parent directory is rejected
// Creating a task list in a directory that doesn't exist should return FailedPrecondition.
// **Validates: Requirements 8.2, 9.4**
func TestProperty15_NonExistentParentDirRejected(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")

		// Generate a nested path that doesn't exist
		seg1 := validNameGen().Draw(rt, "seg1")
		seg2 := validNameGen().Draw(rt, "seg2")
		nestedPath := filepath.Join(seg1, seg2)

		_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title:     name,
			ParentDir: nestedPath,
			Tasks:     []*pb.MainTask{{Description: "task in nested dir"}},
		})
		if err == nil {
			rt.Fatalf("expected NotFound for non-existent parent dir %q, got nil", nestedPath)
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			rt.Fatalf("expected NotFound, got %v", connect.CodeOf(err))
		}

		// Verify directories were NOT created
		dirPath := filepath.Join(tmp, seg1)
		if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
			rt.Fatalf("directory %q should not have been created", dirPath)
		}
	})
}

// Feature: task-management, Property 16: Delete removes task list from disk
// **Validates: Requirements 3.5**
func TestProperty16_DeleteRemovesTaskListFromDisk(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: []*pb.MainTask{{Description: "to be deleted"}},
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		_, err = srv.DeleteTaskList(context.Background(), &pb.DeleteTaskListRequest{
			Id: createResp.TaskList.Id,
		})
		if err != nil {
			rt.Fatalf("DeleteTaskList failed: %v", err)
		}

		// File must not exist
		expectedFile := "tasks_" + name + ".md"
		absPath := filepath.Join(tmp, expectedFile)
		if _, err := os.Stat(absPath); !os.IsNotExist(err) {
			rt.Fatalf("expected file %q to be deleted", absPath)
		}

		// GetTaskList must return NotFound
		_, err = srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			Id: createResp.TaskList.Id,
		})
		if err == nil {
			rt.Fatal("expected NotFound after delete, got nil")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			rt.Fatalf("expected NotFound, got %v", connect.CodeOf(err))
		}
	})
}

// Feature: tasklist-stable-ids, Property 2: Created id is valid UUIDv4
// For any valid title and tasks, the id field in the TaskList returned by
// CreateTaskList shall be a lowercase hyphenated UUIDv4 string.
// **Validates: Requirements 1.2**
func TestProperty2_CreatedIdIsValidUuidV4(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		resp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		id := resp.TaskList.Id
		if err := common.ValidateUuidV4(id); err != nil {
			rt.Fatalf("returned id %q is not a valid UUIDv4: %v", id, err)
		}
	})
}

// Feature: tasklist-stable-ids, Property 3: All created ids are unique
// For any sequence of N valid CreateTaskList calls (with distinct titles),
// all N returned id values shall be pairwise distinct.
// **Validates: Requirements 1.3**
func TestProperty3_AllCreatedIdsAreUnique(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		n := rapid.IntRange(2, 10).Draw(rt, "n")

		seen := make(map[string]bool, n)
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("list_%d_%s", i, validNameGen().Draw(rt, fmt.Sprintf("name-%d", i)))
			tasks := simpleTaskListGen().Draw(rt, fmt.Sprintf("tasks-%d", i))

			resp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
				Title: name,
				Tasks: tasks,
			})
			if err != nil {
				rt.Fatalf("CreateTaskList[%d] failed: %v", i, err)
			}

			id := resp.TaskList.Id
			if seen[id] {
				rt.Fatalf("duplicate id %q at index %d", id, i)
			}
			seen[id] = true
		}
	})
}

// Feature: tasklist-stable-ids, Property 4: Update by id preserves the TaskList_Id
// For any created task list and any new valid tasks, calling UpdateTaskList with
// the task list's id shall return a TaskList whose id is identical to the original.
// **Validates: Requirements 1.4, 5.1**
func TestProperty4_UpdateByIdPreservesTaskListId(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		originalId := createResp.TaskList.Id

		newTasks := simpleTaskListGen().Draw(rt, "newTasks")
		updateResp, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
			Id:    originalId,
			Title: name,
			Tasks: newTasks,
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList failed: %v", err)
		}

		if updateResp.TaskList.Id != originalId {
			rt.Fatalf("id changed after update: expected %q, got %q", originalId, updateResp.TaskList.Id)
		}
	})
}


// Feature: tasklist-stable-ids, Property 5: Delete by id removes file and registry entry
// For any created task list, calling DeleteTaskList with the task list's id shall
// succeed, and a subsequent GetTaskList with the same id shall return NotFound.
// **Validates: Requirements 2.2, 6.1**
func TestProperty5_DeleteByIdRemovesFileAndRegistryEntry(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		id := createResp.TaskList.Id

		_, err = srv.DeleteTaskList(context.Background(), &pb.DeleteTaskListRequest{
			Id: id,
		})
		if err != nil {
			rt.Fatalf("DeleteTaskList failed: %v", err)
		}

		// File must not exist on disk
		expectedFile := "tasks_" + name + ".md"
		absPath := filepath.Join(tmp, expectedFile)
		if _, err := os.Stat(absPath); !os.IsNotExist(err) {
			rt.Fatalf("expected file %q to be deleted", absPath)
		}

		// GetTaskList must return NotFound
		_, err = srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			Id: id,
		})
		if err == nil {
			rt.Fatal("expected NotFound after delete, got nil")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			rt.Fatalf("expected NotFound, got %v", connect.CodeOf(err))
		}
	})
}


// Feature: tasklist-stable-ids, Property 7: Create then list includes the created task list's id
// For any valid title and tasks, creating a task list via CreateTaskList and then
// calling ListTaskLists shall return a list containing a TaskList whose id matches
// the one returned by CreateTaskList.
// **Validates: Requirements 7.1, 10.2**
func TestProperty7_CreateThenListIncludesCreatedId(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		createdId := createResp.TaskList.Id

		listResp, err := srv.ListTaskLists(context.Background(), &pb.ListTaskListsRequest{
			ParentDir: "",
		})
		if err != nil {
			rt.Fatalf("ListTaskLists failed: %v", err)
		}

		found := false
		for _, tl := range listResp.TaskLists {
			if tl.Id == createdId {
				found = true
				break
			}
		}
		if !found {
			rt.Fatalf("ListTaskLists did not include task list with id %q", createdId)
		}
	})
}

// Feature: tasklist-stable-ids, Property 1: Create-then-get round trip
// For any valid title and tasks, creating a task list via CreateTaskList and then
// retrieving it via GetTaskList using the returned id shall produce a TaskList with
// the same id, title, tasks, and file_path fields.
// **Validates: Requirements 1.1, 2.1, 4.1, 8.1, 8.2, 10.1**
func TestProperty1_CreateThenGetRoundTripWithId(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		tmp := t.TempDir()
		srv := NewTaskServer(tmp, testDB(t), nopLogger())
		name := validNameGen().Draw(rt, "name")
		tasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
			Title: name,
			Tasks: tasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		created := createResp.TaskList
		if created.Id == "" {
			rt.Fatal("CreateTaskList returned empty id")
		}

		getResp, err := srv.GetTaskList(context.Background(), &pb.GetTaskListRequest{
			Id: created.Id,
		})
		if err != nil {
			rt.Fatalf("GetTaskList failed: %v", err)
		}

		got := getResp.TaskList

		// Verify id round-trips
		if got.Id != created.Id {
			rt.Fatalf("id mismatch: expected %q, got %q", created.Id, got.Id)
		}

		// Verify title round-trips
		if got.Title != created.Title {
			rt.Fatalf("title mismatch: expected %q, got %q", created.Title, got.Title)
		}

		// Verify parent_dir round-trips
		if got.ParentDir != created.ParentDir {
			rt.Fatalf("parent_dir mismatch: expected %q, got %q", created.ParentDir, got.ParentDir)
		}

		// Verify tasks round-trip
		if len(got.Tasks) != len(created.Tasks) {
			rt.Fatalf("task count mismatch: expected %d, got %d", len(created.Tasks), len(got.Tasks))
		}
		for i, gotTask := range got.Tasks {
			wantTask := created.Tasks[i]
			if gotTask.Description != wantTask.Description {
				rt.Fatalf("task %d description: expected %q, got %q", i, wantTask.Description, gotTask.Description)
			}
			if gotTask.IsDone != wantTask.IsDone {
				rt.Fatalf("task %d done: expected %v, got %v", i, wantTask.IsDone, gotTask.IsDone)
			}
			if len(gotTask.SubTasks) != len(wantTask.SubTasks) {
				rt.Fatalf("task %d subtask count: expected %d, got %d", i, len(wantTask.SubTasks), len(gotTask.SubTasks))
			}
			for j, gs := range gotTask.SubTasks {
				ws := wantTask.SubTasks[j]
				if gs.Description != ws.Description || gs.IsDone != ws.IsDone {
					rt.Fatalf("task %d subtask %d mismatch", i, j)
				}
			}
		}
	})
}


// Feature: tasklist-stable-ids, Property 6: Non-existent id returns NotFound
// For any valid UUIDv4 string that was never used in a CreateTaskList call,
// calling GetTaskList, UpdateTaskList, and DeleteTaskList with that id shall
// return a NotFound error.
// **Validates: Requirements 4.2, 5.2, 6.2**
func TestProperty6_NonExistentIdReturnsNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		id := uuidV4Gen().Draw(rt, "id")
		tmpDir := t.TempDir()
		srv := NewTaskServer(tmpDir, testDB(t), nopLogger())
		ctx := context.Background()

		// GetTaskList with non-existent id should return CodeNotFound
		_, err := srv.GetTaskList(ctx, &pb.GetTaskListRequest{
			Id: id,
		})
		assertCodeNotFound(rt, err, "GetTaskList", id)

		// UpdateTaskList with non-existent id should return CodeNotFound
		_, err = srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
			Id:    id,
			Title: "some-title",
			Tasks: []*pb.MainTask{{Description: "some task"}},
		})
		assertCodeNotFound(rt, err, "UpdateTaskList", id)

		// DeleteTaskList with non-existent id should return CodeNotFound
		_, err = srv.DeleteTaskList(ctx, &pb.DeleteTaskListRequest{
			Id: id,
		})
		assertCodeNotFound(rt, err, "DeleteTaskList", id)
	})
}

// Feature: tasklist-stable-ids, Property 8: Invalid UUID returns InvalidArgument
// For any string that is not a valid UUIDv4, calling GetTaskList, UpdateTaskList,
// or DeleteTaskList with that string as the id shall return an InvalidArgument error.
// **Validates: Requirements 9.1, 9.2**
func TestProperty8_InvalidUuidRejectedByRPCs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		badId := invalidUuidGen().Draw(rt, "invalidUuid")
		tmpDir := t.TempDir()
		srv := NewTaskServer(tmpDir, testDB(t), nopLogger())
		ctx := context.Background()

		// GetTaskList with invalid UUID should return CodeInvalidArgument
		_, err := srv.GetTaskList(ctx, &pb.GetTaskListRequest{
			Id: badId,
		})
		assertCodeInvalidArgument(rt, err, "GetTaskList", badId)

		// UpdateTaskList with invalid UUID should return CodeInvalidArgument
		_, err = srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
			Id:    badId,
			Title: "some-title",
			Tasks: []*pb.MainTask{{Description: "some task"}},
		})
		assertCodeInvalidArgument(rt, err, "UpdateTaskList", badId)

		// DeleteTaskList with invalid UUID should return CodeInvalidArgument
		_, err = srv.DeleteTaskList(ctx, &pb.DeleteTaskListRequest{
			Id: badId,
		})
		assertCodeInvalidArgument(rt, err, "DeleteTaskList", badId)
	})
}
