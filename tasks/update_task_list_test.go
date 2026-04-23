package tasks

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "echolist-backend/proto/gen/tasks/v1"
)

func TestUpdateTaskList_RenamesFileAndUpdatesRegistry(t *testing.T) {
	tmp := t.TempDir()
	s := NewTaskServer(tmp, testDB(t), nopLogger())

	createResp, err := s.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
		Title: "old",
		Tasks: []*pb.MainTask{{Description: "first"}},
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	taskListID := createResp.TaskList.Id

	resp, err := s.UpdateTaskList(context.Background(), &pb.UpdateTaskListRequest{
		Id:    taskListID,
		Title: "new",
		Tasks: []*pb.MainTask{{Description: "updated"}},
	})
	if err != nil {
		t.Fatalf("UpdateTaskList failed: %v", err)
	}

	if resp.TaskList.Id != taskListID {
		t.Fatalf("unexpected Id: got %s, want %s", resp.TaskList.Id, taskListID)
	}
	if resp.TaskList.Title != "new" {
		t.Fatalf("unexpected Title: got %s, want %s", resp.TaskList.Title, "new")
	}
	if resp.TaskList.ParentDir != "" {
		t.Fatalf("unexpected ParentDir: got %s, want empty", resp.TaskList.ParentDir)
	}

	// Old file should be gone
	oldFile := filepath.Join(tmp, "tasks_old.md")
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old file to be gone, stat err=%v", err)
	}

	// New file should exist
	newFile := filepath.Join(tmp, "tasks_new.md")
	data, err := os.ReadFile(newFile)
	if err != nil {
		t.Fatalf("reading updated task file failed: %v", err)
	}
	if string(data) == "" {
		t.Fatal("expected updated task file to be non-empty")
	}

	got, err := s.GetTaskList(context.Background(), &pb.GetTaskListRequest{Id: taskListID})
	if err != nil {
		t.Fatalf("GetTaskList after update failed: %v", err)
	}
	if got.TaskList.Title != "new" {
		t.Fatalf("unexpected title after get: got %s, want %s", got.TaskList.Title, "new")
	}
	if got.TaskList.ParentDir != "" {
		t.Fatalf("unexpected parent_dir after get: got %s, want empty", got.TaskList.ParentDir)
	}
	if len(got.TaskList.Tasks) != 1 || got.TaskList.Tasks[0].Description != "updated" {
		t.Fatalf("unexpected tasks after get: %+v", got.TaskList.Tasks)
	}
}
