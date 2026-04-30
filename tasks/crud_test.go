package tasks_test

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/common"
	"echolist-backend/tasks"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func TestCreateTaskList_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	resp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Groceries",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "Buy milk", IsDone: false},
			{
				Description: "Buy vegetables",
				IsDone:      false,
				SubTasks: []*pb.SubTask{
					{Description: "Carrots", IsDone: false},
				},
			},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	tl := resp.TaskList
	if tl.Id == "" {
		t.Fatal("expected non-empty TaskList.Id")
	}
	if tl.Title != "Groceries" {
		t.Fatalf("expected title 'Groceries', got %q", tl.Title)
	}
	if tl.ParentDir != "" {
		t.Fatalf("expected empty ParentDir, got %q", tl.ParentDir)
	}
	if len(tl.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tl.Tasks))
	}

	for i, mt := range tl.Tasks {
		if mt.Id == "" {
			t.Fatalf("task %d: expected non-empty Id", i)
		}
		if err := common.ValidateUuidV4(mt.Id); err != nil {
			t.Fatalf("task %d: invalid UUIDv4: %v", i, err)
		}
	}

	// Verify subtask ID
	if len(tl.Tasks[1].SubTasks) != 1 {
		t.Fatalf("expected 1 subtask on task 1, got %d", len(tl.Tasks[1].SubTasks))
	}
	st := tl.Tasks[1].SubTasks[0]
	if st.Id == "" {
		t.Fatal("subtask: expected non-empty Id")
	}
	if err := common.ValidateUuidV4(st.Id); err != nil {
		t.Fatalf("subtask: invalid UUIDv4: %v", err)
	}
}

func TestGetTaskList_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Shopping",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "Apples", IsDone: false},
			{Description: "Bread", IsDone: true},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	getResp, err := srv.GetTaskList(ctx, &pb.GetTaskListRequest{
		Id: createResp.TaskList.Id,
	})
	if err != nil {
		t.Fatalf("GetTaskList failed: %v", err)
	}

	got := getResp.TaskList
	if got.Id != createResp.TaskList.Id {
		t.Fatalf("Id mismatch: got %q, want %q", got.Id, createResp.TaskList.Id)
	}
	if got.Title != "Shopping" {
		t.Fatalf("Title mismatch: got %q, want %q", got.Title, "Shopping")
	}
	if got.ParentDir != "" {
		t.Fatalf("ParentDir mismatch: got %q, want %q", got.ParentDir, "")
	}
	if len(got.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(got.Tasks))
	}
	if got.Tasks[0].Description != "Apples" {
		t.Fatalf("task 0 description: got %q, want %q", got.Tasks[0].Description, "Apples")
	}
	if got.Tasks[0].IsDone != false {
		t.Fatal("task 0 should not be done")
	}
	if got.Tasks[1].Description != "Bread" {
		t.Fatalf("task 1 description: got %q, want %q", got.Tasks[1].Description, "Bread")
	}
	if got.Tasks[1].IsDone != true {
		t.Fatal("task 1 should be done")
	}
}

func TestGetTaskList_NotFound(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	// Use a valid but non-existent UUID
	_, err := srv.GetTaskList(ctx, &pb.GetTaskListRequest{
		Id: "00000000-0000-4000-a000-000000000000",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestUpdateTaskList_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Original",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "Task A", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	updateResp, err := srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
		Id:    createResp.TaskList.Id,
		Title: "Updated",
		Tasks: []*pb.MainTask{
			{Description: "Task B", IsDone: true},
			{Description: "Task C", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("UpdateTaskList failed: %v", err)
	}

	got := updateResp.TaskList
	if got.Id != createResp.TaskList.Id {
		t.Fatalf("Id mismatch: got %q, want %q", got.Id, createResp.TaskList.Id)
	}
	if got.Title != "Updated" {
		t.Fatalf("Title mismatch: got %q, want %q", got.Title, "Updated")
	}
	if len(got.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(got.Tasks))
	}
	if got.Tasks[0].Description != "Task B" {
		t.Fatalf("task 0 description: got %q, want %q", got.Tasks[0].Description, "Task B")
	}
	if got.Tasks[1].Description != "Task C" {
		t.Fatalf("task 1 description: got %q, want %q", got.Tasks[1].Description, "Task C")
	}
}

func TestUpdateTaskList_PreservesExistingIDs(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Preserve IDs",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "Task 1", IsDone: false},
			{
				Description: "Task 2",
				IsDone:      false,
				SubTasks: []*pb.SubTask{
					{Description: "Sub 1", IsDone: false},
				},
			},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	created := createResp.TaskList
	task1Id := created.Tasks[0].Id
	task2Id := created.Tasks[1].Id
	sub1Id := created.Tasks[1].SubTasks[0].Id

	// Update sending the same IDs back
	updateResp, err := srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
		Id:    created.Id,
		Title: "Preserve IDs",
		Tasks: []*pb.MainTask{
			{Id: task1Id, Description: "Task 1 updated", IsDone: true},
			{
				Id:          task2Id,
				Description: "Task 2 updated",
				IsDone:      false,
				SubTasks: []*pb.SubTask{
					{Id: sub1Id, Description: "Sub 1 updated", IsDone: true},
				},
			},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("UpdateTaskList failed: %v", err)
	}

	updated := updateResp.TaskList
	if updated.Tasks[0].Id != task1Id {
		t.Fatalf("task 1 ID changed: got %q, want %q", updated.Tasks[0].Id, task1Id)
	}
	if updated.Tasks[1].Id != task2Id {
		t.Fatalf("task 2 ID changed: got %q, want %q", updated.Tasks[1].Id, task2Id)
	}
	if updated.Tasks[1].SubTasks[0].Id != sub1Id {
		t.Fatalf("subtask ID changed: got %q, want %q", updated.Tasks[1].SubTasks[0].Id, sub1Id)
	}
}

func TestUpdateTaskList_AssignsNewIDs(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "New IDs",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "Original task", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	// Update with tasks that have empty Id fields
	updateResp, err := srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
		Id:    createResp.TaskList.Id,
		Title: "New IDs",
		Tasks: []*pb.MainTask{
			{Id: "", Description: "New task 1", IsDone: false},
			{
				Id:          "",
				Description: "New task 2",
				IsDone:      false,
				SubTasks: []*pb.SubTask{
					{Id: "", Description: "New sub", IsDone: false},
				},
			},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("UpdateTaskList failed: %v", err)
	}

	updated := updateResp.TaskList
	for i, mt := range updated.Tasks {
		if mt.Id == "" {
			t.Fatalf("task %d: expected non-empty Id", i)
		}
		if err := common.ValidateUuidV4(mt.Id); err != nil {
			t.Fatalf("task %d: invalid UUIDv4: %v", i, err)
		}
	}
	if len(updated.Tasks[1].SubTasks) != 1 {
		t.Fatalf("expected 1 subtask, got %d", len(updated.Tasks[1].SubTasks))
	}
	st := updated.Tasks[1].SubTasks[0]
	if st.Id == "" {
		t.Fatal("subtask: expected non-empty Id")
	}
	if err := common.ValidateUuidV4(st.Id); err != nil {
		t.Fatalf("subtask: invalid UUIDv4: %v", err)
	}
}

func TestDeleteTaskList_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "To Delete",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "Doomed task", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	_, err = srv.DeleteTaskList(ctx, &pb.DeleteTaskListRequest{
		Id: createResp.TaskList.Id,
	})
	if err != nil {
		t.Fatalf("DeleteTaskList failed: %v", err)
	}

	// Verify GetTaskList returns NotFound
	_, err = srv.GetTaskList(ctx, &pb.GetTaskListRequest{
		Id: createResp.TaskList.Id,
	})
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestListTaskLists_Success(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	titles := []string{"List A", "List B", "List C"}
	for _, title := range titles {
		_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
			Title:        title,
			ParentDir:    "",
			Tasks:        []*pb.MainTask{{Description: "task", IsDone: false}},
			IsAutoDelete: false,
		})
		if err != nil {
			t.Fatalf("CreateTaskList(%q) failed: %v", title, err)
		}
	}

	listResp, err := srv.ListTaskLists(ctx, &pb.ListTaskListsRequest{
		ParentDir: "",
	})
	if err != nil {
		t.Fatalf("ListTaskLists failed: %v", err)
	}

	if len(listResp.TaskLists) != 3 {
		t.Fatalf("expected 3 task lists, got %d", len(listResp.TaskLists))
	}
}

func TestListTaskLists_FiltersByParentDir(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	// Create subdirectories on disk
	if err := os.Mkdir(dataDir+"/dirA", 0o755); err != nil {
		t.Fatalf("failed to create dirA: %v", err)
	}
	if err := os.Mkdir(dataDir+"/dirB", 0o755); err != nil {
		t.Fatalf("failed to create dirB: %v", err)
	}

	// Create task lists in different dirs
	_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:        "In DirA 1",
		ParentDir:    "dirA",
		Tasks:        []*pb.MainTask{{Description: "task", IsDone: false}},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList in dirA failed: %v", err)
	}
	_, err = srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:        "In DirA 2",
		ParentDir:    "dirA",
		Tasks:        []*pb.MainTask{{Description: "task", IsDone: false}},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList in dirA failed: %v", err)
	}
	_, err = srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:        "In DirB",
		ParentDir:    "dirB",
		Tasks:        []*pb.MainTask{{Description: "task", IsDone: false}},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList in dirB failed: %v", err)
	}

	// List only dirA
	listResp, err := srv.ListTaskLists(ctx, &pb.ListTaskListsRequest{
		ParentDir: "dirA",
	})
	if err != nil {
		t.Fatalf("ListTaskLists(dirA) failed: %v", err)
	}
	if len(listResp.TaskLists) != 2 {
		t.Fatalf("expected 2 task lists in dirA, got %d", len(listResp.TaskLists))
	}

	// List only dirB
	listResp, err = srv.ListTaskLists(ctx, &pb.ListTaskListsRequest{
		ParentDir: "dirB",
	})
	if err != nil {
		t.Fatalf("ListTaskLists(dirB) failed: %v", err)
	}
	if len(listResp.TaskLists) != 1 {
		t.Fatalf("expected 1 task list in dirB, got %d", len(listResp.TaskLists))
	}
}
