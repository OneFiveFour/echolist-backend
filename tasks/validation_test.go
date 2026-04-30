package tasks_test

import (
	"context"
	"os"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/tasks"
	pb "echolist-backend/proto/gen/tasks/v1"
)

func TestCreateTaskList_EmptyTitleRejected(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "",
		ParentDir: "",
		Tasks:     []*pb.MainTask{{Description: "task", IsDone: false}},
	})
	if err == nil {
		t.Fatal("expected error for empty title, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateTaskList_NullByteTitleRejected(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "bad\x00title",
		ParentDir: "",
		Tasks:     []*pb.MainTask{{Description: "task", IsDone: false}},
	})
	if err == nil {
		t.Fatal("expected error for null byte in title, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateTaskList_PathTraversalRejected(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Legit",
		ParentDir: "../etc",
		Tasks:     []*pb.MainTask{{Description: "task", IsDone: false}},
	})
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	code := connect.CodeOf(err)
	if code != connect.CodeInvalidArgument && code != connect.CodeNotFound {
		t.Fatalf("expected CodeInvalidArgument or CodeNotFound, got %v", code)
	}
}

func TestGetTaskList_InvalidUUIDRejected(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.GetTaskList(ctx, &pb.GetTaskListRequest{
		Id: "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestUpdateTaskList_InvalidUUIDRejected(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
		Id:    "not-a-uuid",
		Title: "Valid Title",
		Tasks: []*pb.MainTask{{Description: "task", IsDone: false}},
	})
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestDeleteTaskList_InvalidUUIDRejected(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.DeleteTaskList(ctx, &pb.DeleteTaskListRequest{
		Id: "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error for invalid UUID, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestUpdateTaskList_InvalidTaskIDRejected(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	// First create a valid task list
	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Test List",
		ParentDir: "",
		Tasks:     []*pb.MainTask{{Description: "task", IsDone: false}},
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	// Update with a task carrying an invalid non-empty ID
	_, err = srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
		Id:    createResp.TaskList.Id,
		Title: "Test List",
		Tasks: []*pb.MainTask{
			{Id: "bad-id", Description: "task", IsDone: false},
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid task ID, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateTaskList_AutoDeleteFiltersDoneTasks(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	// Create a task list with IsAutoDelete=false and 3 tasks
	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Auto Delete Test",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "done non-recurring", IsDone: true},
			{Description: "done recurring", IsDone: true, Recurrence: "FREQ=DAILY"},
			{Description: "open task", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	listID := createResp.TaskList.Id

	// Get the task IDs from the created list
	getResp, err := srv.GetTaskList(ctx, &pb.GetTaskListRequest{Id: listID})
	if err != nil {
		t.Fatalf("GetTaskList failed: %v", err)
	}
	createdTasks := getResp.TaskList.Tasks

	// Now update with IsAutoDelete=true, marking the done non-recurring task as done
	// and the done recurring task as done
	_, err = srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
		Id:    listID,
		Title: "Auto Delete Test",
		Tasks: []*pb.MainTask{
			{Id: createdTasks[0].Id, Description: "done non-recurring", IsDone: true},
			{Id: createdTasks[1].Id, Description: "done recurring", IsDone: true, Recurrence: "FREQ=DAILY"},
			{Id: createdTasks[2].Id, Description: "open task", IsDone: false},
		},
		IsAutoDelete: true,
	})
	if err != nil {
		t.Fatalf("UpdateTaskList failed: %v", err)
	}

	// Get the task list again to verify filtering
	getResp, err = srv.GetTaskList(ctx, &pb.GetTaskListRequest{Id: listID})
	if err != nil {
		t.Fatalf("GetTaskList after auto-delete failed: %v", err)
	}

	resultTasks := getResp.TaskList.Tasks

	// Should have 2 tasks: the recurring one (advanced) and the open one
	if len(resultTasks) != 2 {
		t.Fatalf("expected 2 tasks after auto-delete, got %d", len(resultTasks))
	}

	// Find the recurring task and the open task
	var foundRecurring, foundOpen bool
	for _, task := range resultTasks {
		switch task.Description {
		case "done recurring":
			foundRecurring = true
			// Recurring task should have been advanced: IsDone reset to false
			if task.IsDone {
				t.Error("recurring task should have IsDone=false after advancement")
			}
			// DueDate should be set
			if task.DueDate == "" {
				t.Error("recurring task should have DueDate set after advancement")
			}
		case "open task":
			foundOpen = true
		case "done non-recurring":
			t.Error("done non-recurring task should have been filtered out")
		}
	}

	if !foundRecurring {
		t.Error("recurring task should be preserved")
	}
	if !foundOpen {
		t.Error("open task should be preserved")
	}
}

func TestUpdateTaskList_RecurringTaskAdvancement(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	// Create a task list with a recurring task
	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Recurring Test",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "daily task", IsDone: false, Recurrence: "FREQ=DAILY"},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	listID := createResp.TaskList.Id
	originalDueDate := createResp.TaskList.Tasks[0].DueDate
	taskID := createResp.TaskList.Tasks[0].Id

	// Update marking the recurring task as done
	updateResp, err := srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
		Id:    listID,
		Title: "Recurring Test",
		Tasks: []*pb.MainTask{
			{Id: taskID, Description: "daily task", IsDone: true, Recurrence: "FREQ=DAILY"},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("UpdateTaskList failed: %v", err)
	}

	advancedTask := updateResp.TaskList.Tasks[0]

	// IsDone should be reset to false
	if advancedTask.IsDone {
		t.Error("recurring task IsDone should be reset to false after advancement")
	}

	// DueDate should have advanced (be different from original)
	if advancedTask.DueDate == originalDueDate {
		t.Errorf("recurring task DueDate should have advanced, but still %q", advancedTask.DueDate)
	}

	// DueDate should not be empty
	if advancedTask.DueDate == "" {
		t.Error("recurring task DueDate should not be empty after advancement")
	}
}

func TestCreateTaskList_MutualExclusionDueDateRecurrence(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Mutual Exclusion",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "bad task", IsDone: false, DueDate: "2025-01-01", Recurrence: "FREQ=DAILY"},
		},
	})
	if err == nil {
		t.Fatal("expected error for mutual exclusion of DueDate and Recurrence, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateTaskList_InvalidRRuleRejected(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Bad RRule",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "bad recurrence", IsDone: false, Recurrence: "NOT_VALID"},
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid RRULE, got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestCreateTaskList_NonExistentParentDirRejected(t *testing.T) {
	dataDir := t.TempDir()
	srv := tasks.NewTaskServer(dataDir, tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	// Ensure the directory does NOT exist
	dirPath := dataDir + "/nonexistent/path"
	if _, err := os.Stat(dirPath); err == nil {
		t.Fatal("expected directory to not exist")
	}

	_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Orphan",
		ParentDir: "nonexistent/path",
		Tasks:     []*pb.MainTask{{Description: "task", IsDone: false}},
	})
	if err == nil {
		t.Fatal("expected error for non-existent parent dir, got nil")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}
