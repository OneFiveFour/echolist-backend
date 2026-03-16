package tasks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/common"
	pb "echolist-backend/proto/gen/tasks/v1"
)

// ---------------------------------------------------------------------------
// Unit tests for validateTasks limits
// ---------------------------------------------------------------------------

func TestValidateTasks_TooManyTasks(t *testing.T) {
	tasks := make([]MainTask, common.MaxTasksPerList+1)
	for i := range tasks {
		tasks[i] = MainTask{Description: "task"}
	}

	err := validateTasks(tasks)
	if err == nil {
		t.Fatal("expected error for too many tasks")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

func TestValidateTasks_AtTaskLimit(t *testing.T) {
	tasks := make([]MainTask, common.MaxTasksPerList)
	for i := range tasks {
		tasks[i] = MainTask{Description: "task"}
	}

	if err := validateTasks(tasks); err != nil {
		t.Fatalf("tasks at exact limit should pass: %v", err)
	}
}

func TestValidateTasks_DescriptionTooLong(t *testing.T) {
	tasks := []MainTask{
		{Description: strings.Repeat("x", common.MaxTaskDescriptionBytes+1)},
	}

	err := validateTasks(tasks)
	if err == nil {
		t.Fatal("expected error for oversized description")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

func TestValidateTasks_TooManySubtasks(t *testing.T) {
	subs := make([]SubTask, common.MaxSubtasksPerTask+1)
	for i := range subs {
		subs[i] = SubTask{Description: "sub"}
	}

	tasks := []MainTask{
		{Description: "parent", SubTasks: subs},
	}

	err := validateTasks(tasks)
	if err == nil {
		t.Fatal("expected error for too many subtasks")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

func TestValidateTasks_SubtaskDescriptionTooLong(t *testing.T) {
	tasks := []MainTask{
		{
			Description: "parent",
			SubTasks: []SubTask{
				{Description: strings.Repeat("y", common.MaxSubtaskDescriptionBytes+1)},
			},
		},
	}

	err := validateTasks(tasks)
	if err == nil {
		t.Fatal("expected error for oversized subtask description")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

// ---------------------------------------------------------------------------
// Integration: limits enforced through RPC handlers
// ---------------------------------------------------------------------------

func TestCreateTaskList_DescriptionTooLong(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewTaskServer(dataDir, nopLogger())

	_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
		Title:     "test",
		ParentDir: "",
		Tasks: []*pb.MainTask{
			{Description: strings.Repeat("z", common.MaxTaskDescriptionBytes+1)},
		},
	})
	if err == nil {
		t.Fatal("expected error for oversized task description")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}

func TestUpdateTaskList_TooManyTasks(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewTaskServer(dataDir, nopLogger())

	// Create a valid task file first
	filePath := filepath.Join(dataDir, "tasks_test.md")
	if err := os.WriteFile(filePath, []byte("- [ ] existing task\n"), 0644); err != nil {
		t.Fatal(err)
	}

	bigList := make([]*pb.MainTask, common.MaxTasksPerList+1)
	for i := range bigList {
		bigList[i] = &pb.MainTask{Description: "task"}
	}

	_, err := srv.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
		Id:    "tasks_test.md",
		Tasks:    bigList,
	})
	if err == nil {
		t.Fatal("expected error for too many tasks")
	}

	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", ce.Code())
	}
}
