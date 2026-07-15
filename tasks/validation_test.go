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

	// Create a task list with IsAutoDelete=false and 3 tasks.
	// The recurring task starts with IsDone=false so that the update can trigger
	// the false→true transition needed for recurrence advancement.
	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Auto Delete Test",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "done non-recurring", IsDone: false},
			{Description: "done recurring", IsDone: false, DueDate: "2025-06-01", Recurrence: "FREQ=DAILY"},
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

	// Now update with IsAutoDelete=true, marking the non-recurring and recurring tasks as done.
	// This triggers a false→true transition for both.
	_, err = srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
		Id:    listID,
		Title: "Auto Delete Test",
		Tasks: []*pb.MainTask{
			{Id: createdTasks[0].Id, Description: "done non-recurring", IsDone: true},
			{Id: createdTasks[1].Id, Description: "done recurring", IsDone: true, DueDate: "2025-06-01", Recurrence: "FREQ=DAILY"},
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

	// Create a task list with a recurring task (due_date required with recurrence)
	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Recurring Test",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "daily task", IsDone: false, DueDate: "2025-06-01", Recurrence: "FREQ=DAILY"},
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
			{Id: taskID, Description: "daily task", IsDone: true, DueDate: "2025-06-01", Recurrence: "FREQ=DAILY"},
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

	// DueDate should be exactly one day after the original (FREQ=DAILY)
	if advancedTask.DueDate != "2025-06-02" {
		t.Errorf("expected DueDate 2025-06-02 after daily advancement, got %q", advancedTask.DueDate)
	}
}

func TestUpdateTaskList_AutoDeleteFiltersDoneSubtasks(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	// Create a task list with a main task that has done and open subtasks
	createResp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Subtask Filter Test",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{
				Description: "main task",
				IsDone:      false,
				SubTasks: []*pb.SubTask{
					{Description: "done sub", IsDone: true},
					{Description: "open sub", IsDone: false},
					{Description: "another done sub", IsDone: true},
				},
			},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	listID := createResp.TaskList.Id
	mainTaskID := createResp.TaskList.Tasks[0].Id
	openSubID := createResp.TaskList.Tasks[0].SubTasks[1].Id

	// Update with IsAutoDelete=true — done subtasks should be removed
	_, err = srv.UpdateTaskList(ctx, &pb.UpdateTaskListRequest{
		Id:    listID,
		Title: "Subtask Filter Test",
		Tasks: []*pb.MainTask{
			{
				Id:          mainTaskID,
				Description: "main task",
				IsDone:      false,
				SubTasks: []*pb.SubTask{
					{Id: createResp.TaskList.Tasks[0].SubTasks[0].Id, Description: "done sub", IsDone: true},
					{Id: openSubID, Description: "open sub", IsDone: false},
					{Id: createResp.TaskList.Tasks[0].SubTasks[2].Id, Description: "another done sub", IsDone: true},
				},
			},
		},
		IsAutoDelete: true,
	})
	if err != nil {
		t.Fatalf("UpdateTaskList failed: %v", err)
	}

	// Get the task list and verify subtask filtering
	getResp, err := srv.GetTaskList(ctx, &pb.GetTaskListRequest{Id: listID})
	if err != nil {
		t.Fatalf("GetTaskList failed: %v", err)
	}

	resultTasks := getResp.TaskList.Tasks
	if len(resultTasks) != 1 {
		t.Fatalf("expected 1 main task, got %d", len(resultTasks))
	}

	// The main task should survive (it's not done)
	if resultTasks[0].Description != "main task" {
		t.Fatalf("expected main task description 'main task', got %q", resultTasks[0].Description)
	}

	// Only the open subtask should remain
	if len(resultTasks[0].SubTasks) != 1 {
		t.Fatalf("expected 1 subtask after auto-delete filtering, got %d", len(resultTasks[0].SubTasks))
	}
	if resultTasks[0].SubTasks[0].Description != "open sub" {
		t.Fatalf("expected surviving subtask 'open sub', got %q", resultTasks[0].SubTasks[0].Description)
	}
}

func TestCreateTaskList_DueDateAndRecurrenceAllowed(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	// Under the new semantics, having both DueDate and Recurrence is the normal
	// valid state for a recurring task.
	resp, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Both Fields Set",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "recurring with date", IsDone: false, DueDate: "2025-01-01", Recurrence: "FREQ=DAILY"},
		},
	})
	if err != nil {
		t.Fatalf("expected no error when both DueDate and Recurrence are set, got %v", err)
	}
	if resp.TaskList.Tasks[0].DueDate != "2025-01-01" {
		t.Errorf("expected DueDate to be stored as-is, got %q", resp.TaskList.Tasks[0].DueDate)
	}
	if resp.TaskList.Tasks[0].Recurrence != "FREQ=DAILY" {
		t.Errorf("expected Recurrence to be stored as-is, got %q", resp.TaskList.Tasks[0].Recurrence)
	}
}

func TestCreateTaskList_RecurrenceWithoutDueDateRejected(t *testing.T) {
	srv := tasks.NewTaskServer(t.TempDir(), tasks.NewTestDB(t), tasks.NopLogger())
	ctx := context.Background()

	// Recurrence without due_date must be rejected.
	_, err := srv.CreateTaskList(ctx, &pb.CreateTaskListRequest{
		Title:     "Missing DueDate",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: "bad task", IsDone: false, Recurrence: "FREQ=DAILY"},
		},
	})
	if err == nil {
		t.Fatal("expected error when recurrence is set without due_date, got nil")
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
			{Description: "bad recurrence", IsDone: false, DueDate: "2025-01-01", Recurrence: "NOT_VALID"},
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
