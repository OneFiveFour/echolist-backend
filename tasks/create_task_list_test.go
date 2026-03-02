package tasks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	pb "echolist-backend/proto/gen/tasks/v1"
)

func TestCreateTaskList_NonExistentParentDirRejected(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewTaskServer(dataDir)

	_, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
		Title:     "test",
		ParentDir: "nonexistent/deep/path",
		Tasks:     []*pb.MainTask{{Description: "a task"}},
	})
	if err == nil {
		t.Fatal("expected error for non-existent parent dir, got nil")
	}

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("expected connect.Error, got %T: %v", err, err)
	}
	if connectErr.Code() != connect.CodeFailedPrecondition {
		t.Fatalf("expected CodeFailedPrecondition, got %v", connectErr.Code())
	}

	// Verify no directories were created
	if _, err := os.Stat(filepath.Join(dataDir, "nonexistent")); !os.IsNotExist(err) {
		t.Fatal("directory should not have been created")
	}
}

func TestCreateTaskList_ExistingParentDirSucceeds(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewTaskServer(dataDir)

	// Pre-create the parent directory
	parentDir := filepath.Join(dataDir, "existing")
	if err := os.Mkdir(parentDir, 0755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}

	resp, err := srv.CreateTaskList(context.Background(), &pb.CreateTaskListRequest{
		Title:     "groceries",
		ParentDir: "existing",
		Tasks:     []*pb.MainTask{{Description: "buy milk"}},
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}
	if resp.TaskList.FilePath != "existing/tasks_groceries.md" {
		t.Fatalf("unexpected file_path: %s", resp.TaskList.FilePath)
	}
}
