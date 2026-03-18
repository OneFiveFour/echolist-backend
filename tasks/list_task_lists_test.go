package tasks

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "echolist-backend/proto/gen/tasks/v1"
)

func TestListTaskLists_OrphanFileReturnsEmptyId(t *testing.T) {
	dataDir := t.TempDir()
	srv := NewTaskServer(dataDir, nopLogger())

	// Write a valid task list file directly on disk, bypassing CreateTaskList
	// so no registry entry is created.
	filename := "tasks_Orphan.md"
	content := PrintTaskFile([]MainTask{{Description: "buy milk"}})
	if err := os.WriteFile(filepath.Join(dataDir, filename), content, 0644); err != nil {
		t.Fatalf("failed to write orphan task file: %v", err)
	}

	resp, err := srv.ListTaskLists(context.Background(), &pb.ListTaskListsRequest{
		ParentDir: "",
	})
	if err != nil {
		t.Fatalf("ListTaskLists failed: %v", err)
	}

	if len(resp.TaskLists) != 1 {
		t.Fatalf("expected 1 task list, got %d", len(resp.TaskLists))
	}

	tl := resp.TaskLists[0]
	if tl.Id != "" {
		t.Errorf("expected empty id for orphan task list, got %q", tl.Id)
	}
	if tl.Title != "Orphan" {
		t.Errorf("expected title %q, got %q", "Orphan", tl.Title)
	}
	if len(tl.Tasks) != 1 || tl.Tasks[0].Description != "buy milk" {
		t.Errorf("unexpected tasks: %v", tl.Tasks)
	}
}
