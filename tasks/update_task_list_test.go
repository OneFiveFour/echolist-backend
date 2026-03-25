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
	s := NewTaskServer(tmp, nopLogger())

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
	if resp.TaskList.FilePath != "tasks_new.md" {
		t.Fatalf("unexpected FilePath: got %s, want %s", resp.TaskList.FilePath, "tasks_new.md")
	}

	if _, err := os.Stat(filepath.Join(tmp, createResp.TaskList.FilePath)); !os.IsNotExist(err) {
		t.Fatalf("expected old file to be gone, stat err=%v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmp, resp.TaskList.FilePath))
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
	if got.TaskList.FilePath != "tasks_new.md" {
		t.Fatalf("unexpected file_path after get: got %s, want %s", got.TaskList.FilePath, "tasks_new.md")
	}
	if len(got.TaskList.Tasks) != 1 || got.TaskList.Tasks[0].Description != "updated" {
		t.Fatalf("unexpected tasks after get: %+v", got.TaskList.Tasks)
	}
}
