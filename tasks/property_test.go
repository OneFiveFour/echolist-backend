package tasks_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/common"
	"echolist-backend/tasks"
	pb "echolist-backend/proto/gen/tasks/v1"

	"pgregory.net/rapid"
)

// --- Generators ---

// validNameGen generates valid names (alphanumeric + hyphens/underscores, 1-30 chars).
func validNameGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]{0,29}`)
}

// simpleTaskGen generates a simple MainTask proto (no due date, no recurrence).
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

// invalidUuidGen generates strings that are NOT valid UUIDv4.
func invalidUuidGen() *rapid.Generator[string] {
	return rapid.OneOf(
		rapid.Just(""),
		rapid.StringMatching(`[a-zA-Z0-9]{1,30}`),
		rapid.StringMatching(`[0-9a-f]{32}`), // no hyphens
	)
}

// traversalPathGen generates path traversal strings.
func traversalPathGen() *rapid.Generator[string] {
	return rapid.SampledFrom([]string{
		"../etc/passwd", "../../secret", "foo/../../bar", "../", "foo/../../../etc",
	})
}

// --- Property Tests ---

// Feature: test-suite-sqlite-rewrite, Property 1: Task Create-Then-Get Round Trip
// Validates: Requirements 1.1, 8.1
func TestProperty_TaskCreateThenGetRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
		ctx := context.Background()

		name := validNameGen().Draw(rt, "name")
		inputTasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
			Title:     name,
			ParentDir: "",
			Tasks:     inputTasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		getResp, err := srv.GetTaskList(ctx, &pb.GetTaskListRequest{
			Id: createResp.TaskList.Id,
		})
		if err != nil {
			rt.Fatalf("GetTaskList failed: %v", err)
		}

		got := getResp.TaskList

		// Same title
		if got.Title != name {
			rt.Fatalf("title mismatch: got %q, want %q", got.Title, name)
		}

		// Same task count
		if len(got.Tasks) != len(inputTasks) {
			rt.Fatalf("task count mismatch: got %d, want %d", len(got.Tasks), len(inputTasks))
		}

		// Same descriptions, done states, subtask counts
		for i, task := range got.Tasks {
			if task.Description != inputTasks[i].Description {
				rt.Fatalf("task %d description mismatch: got %q, want %q", i, task.Description, inputTasks[i].Description)
			}
			if task.IsDone != inputTasks[i].IsDone {
				rt.Fatalf("task %d IsDone mismatch: got %v, want %v", i, task.IsDone, inputTasks[i].IsDone)
			}
			if len(task.SubTasks) != len(inputTasks[i].SubTasks) {
				rt.Fatalf("task %d subtask count mismatch: got %d, want %d", i, len(task.SubTasks), len(inputTasks[i].SubTasks))
			}
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 3: All Generated IDs Are Valid UUIDv4
// Validates: Requirements 4.1, 4.2
func TestProperty_AllGeneratedIDsAreValidUUIDv4(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
		ctx := context.Background()

		name := validNameGen().Draw(rt, "name")
		inputTasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
			Title:     name,
			ParentDir: "",
			Tasks:     inputTasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		tl := createResp.TaskList
		allIDs := make(map[string]bool)

		// Verify TaskList.Id
		if err := common.ValidateUuidV4(tl.Id); err != nil {
			rt.Fatalf("TaskList.Id is not valid UUIDv4: %v", err)
		}
		allIDs[tl.Id] = true

		// Verify all MainTask IDs and SubTask IDs
		for i, mt := range tl.Tasks {
			if err := common.ValidateUuidV4(mt.Id); err != nil {
				rt.Fatalf("task %d Id is not valid UUIDv4: %v", i, err)
			}
			if allIDs[mt.Id] {
				rt.Fatalf("duplicate ID found: %s", mt.Id)
			}
			allIDs[mt.Id] = true

			for j, st := range mt.SubTasks {
				if err := common.ValidateUuidV4(st.Id); err != nil {
					rt.Fatalf("task %d subtask %d Id is not valid UUIDv4: %v", i, j, err)
				}
				if allIDs[st.Id] {
					rt.Fatalf("duplicate ID found: %s", st.Id)
				}
				allIDs[st.Id] = true
			}
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 4: Task ID Stability on Update
// Validates: Requirements 4.3, 4.4, 8.2
func TestProperty_TaskIDStabilityOnUpdate(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
		ctx := context.Background()

		name := validNameGen().Draw(rt, "name")
		inputTasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
			Title:     name,
			ParentDir: "",
			Tasks:     inputTasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		created := createResp.TaskList

		// Collect existing IDs
		existingIDs := make([]string, len(created.Tasks))
		for i, mt := range created.Tasks {
			existingIDs[i] = mt.Id
		}

		// Build update request: send existing tasks back with their IDs + one new task with empty ID
		updateTasks := make([]*pb.MainTask, 0, len(created.Tasks)+1)
		for _, mt := range created.Tasks {
			updateTasks = append(updateTasks, &pb.MainTask{
				Id:          mt.Id,
				Description: mt.Description,
				IsDone:      mt.IsDone,
				SubTasks:    mt.SubTasks,
			})
		}
		// Add a new task with empty ID
		updateTasks = append(updateTasks, &pb.MainTask{
			Id:          "",
			Description: "new task",
			IsDone:      false,
		})

		updateResp, err := srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
			Id:    created.Id,
			Title: name,
			Tasks: updateTasks,
		})
		if err != nil {
			rt.Fatalf("UpdateTaskList failed: %v", err)
		}

		updated := updateResp.TaskList

		// Verify existing IDs are preserved
		for i, expectedID := range existingIDs {
			if updated.Tasks[i].Id != expectedID {
				rt.Fatalf("task %d ID changed: got %q, want %q", i, updated.Tasks[i].Id, expectedID)
			}
		}

		// Verify new task got a valid UUIDv4
		newTask := updated.Tasks[len(updated.Tasks)-1]
		if err := common.ValidateUuidV4(newTask.Id); err != nil {
			rt.Fatalf("new task Id is not valid UUIDv4: %v", err)
		}

		// Verify new task ID is different from all existing IDs
		for _, existingID := range existingIDs {
			if newTask.Id == existingID {
				rt.Fatalf("new task ID collides with existing ID: %s", newTask.Id)
			}
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 7: Path Traversal Prevention
// Validates: Requirements 8.5
func TestProperty_PathTraversalPrevention(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
		ctx := context.Background()

		traversal := traversalPathGen().Draw(rt, "traversal")

		// CreateTaskList with traversal parent_dir
		_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
			Title:     "test",
			ParentDir: traversal,
			Tasks:     []*pb.MainTask{{Description: "task", IsDone: false}},
		})
		if err == nil {
			rt.Fatalf("CreateTaskList should reject traversal path %q", traversal)
		}
		code := connect.CodeOf(err)
		if code != connect.CodeInvalidArgument && code != connect.CodeNotFound {
			rt.Fatalf("CreateTaskList(%q): expected InvalidArgument or NotFound, got %v", traversal, code)
		}

		// ListTaskLists with traversal parent_dir
		_, err = srv.ListTaskLists(ctx, &pb.ListTaskListsRequest{
			ParentDir: traversal,
		})
		if err == nil {
			rt.Fatalf("ListTaskLists should reject traversal path %q", traversal)
		}
		code = connect.CodeOf(err)
		if code != connect.CodeInvalidArgument && code != connect.CodeNotFound {
			rt.Fatalf("ListTaskLists(%q): expected InvalidArgument or NotFound, got %v", traversal, code)
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 8: Invalid UUID Rejection
// Validates: Requirements 8.6
func TestProperty_InvalidUUIDRejection(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
		ctx := context.Background()

		invalidID := invalidUuidGen().Draw(rt, "invalidUUID")

		// GetTaskList with invalid UUID
		_, err := srv.GetTaskList(ctx, &pb.GetTaskListRequest{Id: invalidID})
		if err == nil {
			rt.Fatalf("GetTaskList should reject invalid UUID %q", invalidID)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			rt.Fatalf("GetTaskList(%q): expected CodeInvalidArgument, got %v", invalidID, connect.CodeOf(err))
		}

		// UpdateTaskList with invalid UUID
		_, err = srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
			Id:    invalidID,
			Title: "Valid",
			Tasks: []*pb.MainTask{{Description: "task", IsDone: false}},
		})
		if err == nil {
			rt.Fatalf("UpdateTaskList should reject invalid UUID %q", invalidID)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			rt.Fatalf("UpdateTaskList(%q): expected CodeInvalidArgument, got %v", invalidID, connect.CodeOf(err))
		}

		// DeleteTaskList with invalid UUID
		_, err = srv.DeleteTaskList(ctx, &pb.DeleteTaskListRequest{Id: invalidID})
		if err == nil {
			rt.Fatalf("DeleteTaskList should reject invalid UUID %q", invalidID)
		}
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			rt.Fatalf("DeleteTaskList(%q): expected CodeInvalidArgument, got %v", invalidID, connect.CodeOf(err))
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 15: Delete-Then-Get Returns NotFound
// Validates: Requirements 1.4
func TestProperty_DeleteThenGetReturnsNotFound(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
		ctx := context.Background()

		name := validNameGen().Draw(rt, "name")
		inputTasks := simpleTaskListGen().Draw(rt, "tasks")

		createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
			Title:     name,
			ParentDir: "",
			Tasks:     inputTasks,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		id := createResp.TaskList.Id

		// Delete
		_, err = srv.DeleteTaskList(ctx, &pb.DeleteTaskListRequest{Id: id})
		if err != nil {
			rt.Fatalf("DeleteTaskList failed: %v", err)
		}

		// Get should return NotFound
		_, err = srv.GetTaskList(ctx, &pb.GetTaskListRequest{Id: id})
		if err == nil {
			rt.Fatal("GetTaskList should return error after delete")
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			rt.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 17: ListTaskLists Parent Dir Filtering
// Validates: Requirements 8.1
func TestProperty_ListTaskListsParentDirFiltering(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
		ctx := context.Background()

		// Generate 2-3 unique directory names
		numDirs := rapid.IntRange(2, 3).Draw(rt, "numDirs")
		dirs := make([]string, numDirs)
		for i := 0; i < numDirs; i++ {
			for {
				dir := validNameGen().Draw(rt, fmt.Sprintf("dir-%d", i))
				// Ensure uniqueness
				unique := true
				for j := 0; j < i; j++ {
					if strings.EqualFold(dirs[j], dir) {
						unique = false
						break
					}
				}
				if unique {
					dirs[i] = dir
					break
				}
			}
		}

		// Create directories on disk
		for _, dir := range dirs {
			if err := os.MkdirAll(filepath.Join(dataDir, dir), 0o755); err != nil {
				rt.Fatalf("failed to create dir %q: %v", dir, err)
			}
		}

		// Create task lists in each directory (1-2 per dir)
		expectedCounts := make(map[string]int)
		for _, dir := range dirs {
			numLists := rapid.IntRange(1, 2).Draw(rt, fmt.Sprintf("numLists-%s", dir))
			for j := 0; j < numLists; j++ {
				name := validNameGen().Draw(rt, fmt.Sprintf("name-%s-%d", dir, j))
				_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
					Title:     name,
					ParentDir: dir,
					Tasks:     []*pb.MainTask{{Description: "task", IsDone: false}},
				})
				if err != nil {
					rt.Fatalf("CreateTaskList in %q failed: %v", dir, err)
				}
				expectedCounts[dir]++
			}
		}

		// Verify each directory returns only its own task lists
		for _, dir := range dirs {
			listResp, err := srv.ListTaskLists(ctx, &pb.ListTaskListsRequest{
				ParentDir: dir,
			})
			if err != nil {
				rt.Fatalf("ListTaskLists(%q) failed: %v", dir, err)
			}
			if len(listResp.TaskLists) != expectedCounts[dir] {
				rt.Fatalf("ListTaskLists(%q): expected %d, got %d", dir, expectedCounts[dir], len(listResp.TaskLists))
			}
			// Verify all returned items belong to this dir
			for _, tl := range listResp.TaskLists {
				if tl.ParentDir != dir {
					rt.Fatalf("ListTaskLists(%q) returned item with ParentDir=%q", dir, tl.ParentDir)
				}
			}
		}
	})
}
