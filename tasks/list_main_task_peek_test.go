package tasks_test

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/tasks"
	pb "echolist-backend/proto/gen/tasks/v1"
)

// TestListMainTaskPeek_AggregatesAcrossLists verifies that main tasks from all
// task lists directly in a directory are returned in one flat response, grouped
// per task list and ordered within each list by stored position. Subtasks are
// excluded and each entry carries its owning task list's id and name.
func TestListMainTaskPeek_AggregatesAcrossLists(t *testing.T) {
	dataDir := tasks.ResolvedTempDir(t)
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	listA, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "List A",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "A1", IsDone: false, DueDate: "2026-01-01"},
			{
				Description: "A2",
				IsDone:      true,
				SubTasks:    []*pb.SubTask{{Description: "A2-sub", IsDone: false}},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateTaskList(List A) failed: %v", err)
	}

	listB, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "List B",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "B1", IsDone: false},
		},
	})
	if err != nil {
		t.Fatalf("CreateTaskList(List B) failed: %v", err)
	}

	resp, err := srv.ListMainTaskPeek(ctx, &pb.ListMainTaskPeekRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListMainTaskPeek failed: %v", err)
	}

	if len(resp.Tasks) != 3 {
		t.Fatalf("expected 3 main tasks, got %d", len(resp.Tasks))
	}

	// No subtasks should ever appear: every returned id must be a main task id.
	mainTaskIds := map[string]bool{}
	for _, mt := range listA.TaskList.Tasks {
		mainTaskIds[mt.Id] = true
	}
	for _, mt := range listB.TaskList.Tasks {
		mainTaskIds[mt.Id] = true
	}
	for _, peek := range resp.Tasks {
		if !mainTaskIds[peek.MainTaskId] {
			t.Fatalf("returned id %q is not a known main task id (subtask leaked?)", peek.MainTaskId)
		}
	}

	// Build a per-list view to assert grouping and intra-list ordering.
	byList := map[string][]*pb.MainTaskPeek{}
	for _, peek := range resp.Tasks {
		byList[peek.TaskListId] = append(byList[peek.TaskListId], peek)
	}

	gotA := byList[listA.TaskList.Id]
	if len(gotA) != 2 {
		t.Fatalf("expected 2 tasks for List A, got %d", len(gotA))
	}
	if gotA[0].Description != "A1" || gotA[1].Description != "A2" {
		t.Fatalf("List A tasks out of stored order: got %q, %q", gotA[0].Description, gotA[1].Description)
	}
	if gotA[0].TaskListName != "List A" {
		t.Fatalf("expected task_list_name 'List A', got %q", gotA[0].TaskListName)
	}
	if gotA[0].DueDate != "2026-01-01" {
		t.Fatalf("expected due_date '2026-01-01', got %q", gotA[0].DueDate)
	}
	if gotA[1].DueDate != "" {
		t.Fatalf("expected empty due_date for task without one, got %q", gotA[1].DueDate)
	}
	if gotA[1].IsDone != true {
		t.Fatal("expected A2 to be done")
	}

	gotB := byList[listB.TaskList.Id]
	if len(gotB) != 1 {
		t.Fatalf("expected 1 task for List B, got %d", len(gotB))
	}
	if gotB[0].TaskListName != "List B" {
		t.Fatalf("expected task_list_name 'List B', got %q", gotB[0].TaskListName)
	}

	// Grouping: rows for the same task list must be contiguous in the response.
	for i := 1; i < len(resp.Tasks); i++ {
		prev := resp.Tasks[i-1].TaskListId
		cur := resp.Tasks[i].TaskListId
		if prev != cur {
			// a boundary; ensure cur did not already appear before prev's block
			for j := 0; j < i-1; j++ {
				if resp.Tasks[j].TaskListId == cur {
					t.Fatalf("task list %q is not contiguous in response", cur)
				}
			}
		}
	}
}

// TestListMainTaskPeek_DoesNotRecurseIntoSubdirs verifies that only task lists
// directly in the requested directory are aggregated, not those in subdirs.
func TestListMainTaskPeek_DoesNotRecurseIntoSubdirs(t *testing.T) {
	dataDir := tasks.ResolvedTempDir(t)
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	if err := os.Mkdir(dataDir+"/sub", 0o755); err != nil {
		t.Fatalf("failed to create sub dir: %v", err)
	}

	if _, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Root list",
		ParentDir: "",
		Tasks:     []*pb.MainTask{{Description: "root task"}},
	}); err != nil {
		t.Fatalf("CreateTaskList(root) failed: %v", err)
	}
	if _, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Sub list",
		ParentDir: "sub",
		Tasks:     []*pb.MainTask{{Description: "sub task"}},
	}); err != nil {
		t.Fatalf("CreateTaskList(sub) failed: %v", err)
	}

	rootResp, err := srv.ListMainTaskPeek(ctx, &pb.ListMainTaskPeekRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListMainTaskPeek(root) failed: %v", err)
	}
	if len(rootResp.Tasks) != 1 {
		t.Fatalf("expected 1 task at root (no recursion), got %d", len(rootResp.Tasks))
	}
	if rootResp.Tasks[0].Description != "root task" {
		t.Fatalf("expected 'root task', got %q", rootResp.Tasks[0].Description)
	}

	subResp, err := srv.ListMainTaskPeek(ctx, &pb.ListMainTaskPeekRequest{ParentDir: "sub"})
	if err != nil {
		t.Fatalf("ListMainTaskPeek(sub) failed: %v", err)
	}
	if len(subResp.Tasks) != 1 {
		t.Fatalf("expected 1 task in sub, got %d", len(subResp.Tasks))
	}
	if subResp.Tasks[0].Description != "sub task" {
		t.Fatalf("expected 'sub task', got %q", subResp.Tasks[0].Description)
	}
}

// TestListMainTaskPeek_EmptyDirNoLists verifies a directory without task lists
// yields a valid empty response, not an error.
func TestListMainTaskPeek_EmptyDirNoLists(t *testing.T) {
	dataDir := tasks.ResolvedTempDir(t)
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	resp, err := srv.ListMainTaskPeek(ctx, &pb.ListMainTaskPeekRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListMainTaskPeek failed: %v", err)
	}
	if len(resp.Tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(resp.Tasks))
	}
}

// TestListMainTaskPeek_ListWithoutMainTasks verifies a task list that has no
// main tasks contributes nothing and does not error.
func TestListMainTaskPeek_ListWithoutMainTasks(t *testing.T) {
	dataDir := tasks.ResolvedTempDir(t)
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	if _, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Empty list",
		ParentDir: "",
		Tasks:     nil,
	}); err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	resp, err := srv.ListMainTaskPeek(ctx, &pb.ListMainTaskPeekRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListMainTaskPeek failed: %v", err)
	}
	if len(resp.Tasks) != 0 {
		t.Fatalf("expected 0 tasks for a list without main tasks, got %d", len(resp.Tasks))
	}
}

// TestListMainTaskPeek_NonexistentDir verifies a missing directory is reported
// consistently with ListTaskLists as CodeNotFound.
func TestListMainTaskPeek_NonexistentDir(t *testing.T) {
	dataDir := tasks.ResolvedTempDir(t)
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.ListMainTaskPeek(ctx, &pb.ListMainTaskPeekRequest{ParentDir: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

// TestListMainTaskPeek_InvalidParentDir verifies a traversal path is rejected
// with CodeInvalidArgument, consistent with other parent_dir validation.
func TestListMainTaskPeek_InvalidParentDir(t *testing.T) {
	dataDir := tasks.ResolvedTempDir(t)
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.ListMainTaskPeek(ctx, &pb.ListMainTaskPeekRequest{ParentDir: "../escape"})
	if err == nil {
		t.Fatal("expected error for traversal path, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}
