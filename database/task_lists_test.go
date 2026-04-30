package database_test

import (
	"path/filepath"
	"testing"

	"echolist-backend/database"
)

func newTestDB(t *testing.T) *database.Database {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestCreateTaskList(t *testing.T) {
	db := newTestDB(t)

	params := database.CreateTaskListParams{
		Id:           "tl-create-001",
		Title:        "Shopping List",
		ParentDir:    "Home",
		IsAutoDelete: true,
		CreatedAt:    1000,
		UpdatedAt:    1000,
		Tasks: []database.CreateTaskParams{
			{
				Id:          "task-001",
				Description: "Buy milk",
				IsDone:      false,
				SubTasks: []database.CreateTaskParams{
					{
						Id:          "sub-001",
						Description: "Check expiry date",
						IsDone:      false,
					},
					{
						Id:          "sub-002",
						Description: "Compare prices",
						IsDone:      true,
					},
				},
			},
			{
				Id:          "task-002",
				Description: "Buy bread",
				IsDone:      true,
			},
		},
	}

	tl, tasks, err := db.CreateTaskList(params)
	if err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	// Verify TaskListRow fields.
	if tl.Id != "tl-create-001" {
		t.Errorf("expected Id %q, got %q", "tl-create-001", tl.Id)
	}
	if tl.Title != "Shopping List" {
		t.Errorf("expected Title %q, got %q", "Shopping List", tl.Title)
	}
	if tl.ParentDir != "Home" {
		t.Errorf("expected ParentDir %q, got %q", "Home", tl.ParentDir)
	}
	if !tl.IsAutoDelete {
		t.Error("expected IsAutoDelete true, got false")
	}
	if tl.CreatedAt != 1000 {
		t.Errorf("expected CreatedAt 1000, got %d", tl.CreatedAt)
	}
	if tl.UpdatedAt != 1000 {
		t.Errorf("expected UpdatedAt 1000, got %d", tl.UpdatedAt)
	}

	// Verify task count: 2 main + 2 subtasks = 4 total.
	if len(tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(tasks))
	}

	// Verify main tasks have TaskListId set and ParentTaskId nil.
	mainTasks := []database.TaskRow{}
	subTasks := []database.TaskRow{}
	for _, task := range tasks {
		if task.TaskListId != nil {
			mainTasks = append(mainTasks, task)
		} else {
			subTasks = append(subTasks, task)
		}
	}

	if len(mainTasks) != 2 {
		t.Fatalf("expected 2 main tasks, got %d", len(mainTasks))
	}
	if len(subTasks) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(subTasks))
	}

	// Main tasks should have TaskListId set and ParentTaskId nil.
	for _, mt := range mainTasks {
		if mt.TaskListId == nil {
			t.Errorf("main task %s: expected TaskListId non-nil", mt.Id)
		} else if *mt.TaskListId != "tl-create-001" {
			t.Errorf("main task %s: expected TaskListId %q, got %q", mt.Id, "tl-create-001", *mt.TaskListId)
		}
		if mt.ParentTaskId != nil {
			t.Errorf("main task %s: expected ParentTaskId nil, got %q", mt.Id, *mt.ParentTaskId)
		}
	}

	// Subtasks should have ParentTaskId set and TaskListId nil.
	for _, st := range subTasks {
		if st.TaskListId != nil {
			t.Errorf("subtask %s: expected TaskListId nil, got %q", st.Id, *st.TaskListId)
		}
		if st.ParentTaskId == nil {
			t.Errorf("subtask %s: expected ParentTaskId non-nil", st.Id)
		} else if *st.ParentTaskId != "task-001" {
			t.Errorf("subtask %s: expected ParentTaskId %q, got %q", st.Id, "task-001", *st.ParentTaskId)
		}
	}

	// Verify positions are sequential starting from 0.
	if mainTasks[0].Position != 0 {
		t.Errorf("expected first main task position 0, got %d", mainTasks[0].Position)
	}
	if mainTasks[1].Position != 1 {
		t.Errorf("expected second main task position 1, got %d", mainTasks[1].Position)
	}
	if subTasks[0].Position != 0 {
		t.Errorf("expected first subtask position 0, got %d", subTasks[0].Position)
	}
	if subTasks[1].Position != 1 {
		t.Errorf("expected second subtask position 1, got %d", subTasks[1].Position)
	}
}

func TestGetTaskList(t *testing.T) {
	db := newTestDB(t)

	params := database.CreateTaskListParams{
		Id:           "tl-get-001",
		Title:        "Work Tasks",
		ParentDir:    "Office",
		IsAutoDelete: false,
		CreatedAt:    2000,
		UpdatedAt:    2000,
		Tasks: []database.CreateTaskParams{
			{
				Id:          "task-get-001",
				Description: "Write report",
				IsDone:      false,
			},
		},
	}

	created, createdTasks, err := db.CreateTaskList(params)
	if err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	tl, tasks, err := db.GetTaskList("tl-get-001")
	if err != nil {
		t.Fatalf("GetTaskList: %v", err)
	}

	// Verify all fields match the create response.
	if tl.Id != created.Id {
		t.Errorf("Id: expected %q, got %q", created.Id, tl.Id)
	}
	if tl.Title != created.Title {
		t.Errorf("Title: expected %q, got %q", created.Title, tl.Title)
	}
	if tl.ParentDir != created.ParentDir {
		t.Errorf("ParentDir: expected %q, got %q", created.ParentDir, tl.ParentDir)
	}
	if tl.IsAutoDelete != created.IsAutoDelete {
		t.Errorf("IsAutoDelete: expected %v, got %v", created.IsAutoDelete, tl.IsAutoDelete)
	}
	if tl.CreatedAt != created.CreatedAt {
		t.Errorf("CreatedAt: expected %d, got %d", created.CreatedAt, tl.CreatedAt)
	}
	if tl.UpdatedAt != created.UpdatedAt {
		t.Errorf("UpdatedAt: expected %d, got %d", created.UpdatedAt, tl.UpdatedAt)
	}

	if len(tasks) != len(createdTasks) {
		t.Fatalf("expected %d tasks, got %d", len(createdTasks), len(tasks))
	}
	if tasks[0].Id != createdTasks[0].Id {
		t.Errorf("task Id: expected %q, got %q", createdTasks[0].Id, tasks[0].Id)
	}
	if tasks[0].Description != createdTasks[0].Description {
		t.Errorf("task Description: expected %q, got %q", createdTasks[0].Description, tasks[0].Description)
	}
}

func TestGetTaskList_NotFound(t *testing.T) {
	db := newTestDB(t)

	_, _, err := db.GetTaskList("non-existent-id")
	if err != database.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestUpdateTaskList(t *testing.T) {
	db := newTestDB(t)

	// Create initial task list.
	createParams := database.CreateTaskListParams{
		Id:           "tl-update-001",
		Title:        "Old Title",
		ParentDir:    "",
		IsAutoDelete: false,
		CreatedAt:    3000,
		UpdatedAt:    3000,
		Tasks: []database.CreateTaskParams{
			{
				Id:          "old-task-001",
				Description: "Old task",
				IsDone:      false,
			},
		},
	}

	_, _, err := db.CreateTaskList(createParams)
	if err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	// Update with new title and different tasks.
	updateParams := database.UpdateTaskListParams{
		Id:           "tl-update-001",
		Title:        "New Title",
		IsAutoDelete: true,
		UpdatedAt:    4000,
		Tasks: []database.CreateTaskParams{
			{
				Id:          "new-task-001",
				Description: "New task A",
				IsDone:      false,
			},
			{
				Id:          "new-task-002",
				Description: "New task B",
				IsDone:      true,
			},
		},
	}

	tl, tasks, err := db.UpdateTaskList(updateParams)
	if err != nil {
		t.Fatalf("UpdateTaskList: %v", err)
	}

	// Verify title is updated.
	if tl.Title != "New Title" {
		t.Errorf("expected Title %q, got %q", "New Title", tl.Title)
	}

	// Verify UpdatedAt is updated.
	if tl.UpdatedAt != 4000 {
		t.Errorf("expected UpdatedAt 4000, got %d", tl.UpdatedAt)
	}

	// Verify IsAutoDelete is updated.
	if !tl.IsAutoDelete {
		t.Error("expected IsAutoDelete true, got false")
	}

	// Verify old tasks are replaced with new tasks.
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	if tasks[0].Id != "new-task-001" {
		t.Errorf("expected first task Id %q, got %q", "new-task-001", tasks[0].Id)
	}
	if tasks[1].Id != "new-task-002" {
		t.Errorf("expected second task Id %q, got %q", "new-task-002", tasks[1].Id)
	}

	// Verify positions are correct.
	if tasks[0].Position != 0 {
		t.Errorf("expected first task position 0, got %d", tasks[0].Position)
	}
	if tasks[1].Position != 1 {
		t.Errorf("expected second task position 1, got %d", tasks[1].Position)
	}

	// Verify via GetTaskList that old tasks are gone.
	_, getTasks, err := db.GetTaskList("tl-update-001")
	if err != nil {
		t.Fatalf("GetTaskList after update: %v", err)
	}
	for _, task := range getTasks {
		if task.Id == "old-task-001" {
			t.Error("old task should have been replaced but still exists")
		}
	}
}

func TestUpdateTaskList_NotFound(t *testing.T) {
	db := newTestDB(t)

	params := database.UpdateTaskListParams{
		Id:           "non-existent-id",
		Title:        "Title",
		IsAutoDelete: false,
		UpdatedAt:    5000,
		Tasks:        []database.CreateTaskParams{},
	}

	_, _, err := db.UpdateTaskList(params)
	if err != database.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestDeleteTaskList(t *testing.T) {
	db := newTestDB(t)

	params := database.CreateTaskListParams{
		Id:           "tl-delete-001",
		Title:        "To Delete",
		ParentDir:    "",
		IsAutoDelete: false,
		CreatedAt:    6000,
		UpdatedAt:    6000,
		Tasks: []database.CreateTaskParams{
			{
				Id:          "del-task-001",
				Description: "Task to delete",
				IsDone:      false,
			},
		},
	}

	_, _, err := db.CreateTaskList(params)
	if err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	// First delete returns true.
	deleted, err := db.DeleteTaskList("tl-delete-001")
	if err != nil {
		t.Fatalf("first DeleteTaskList: %v", err)
	}
	if !deleted {
		t.Error("expected first delete to return true")
	}

	// Second delete returns false.
	deleted, err = db.DeleteTaskList("tl-delete-001")
	if err != nil {
		t.Fatalf("second DeleteTaskList: %v", err)
	}
	if deleted {
		t.Error("expected second delete to return false")
	}

	// GetTaskList returns ErrNotFound.
	_, _, err = db.GetTaskList("tl-delete-001")
	if err != database.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got: %v", err)
	}
}

func TestListTaskLists(t *testing.T) {
	db := newTestDB(t)

	// Create 3 task lists in the same parent_dir.
	for i, id := range []string{"tl-list-001", "tl-list-002", "tl-list-003"} {
		params := database.CreateTaskListParams{
			Id:           id,
			Title:        "List " + id,
			ParentDir:    "",
			IsAutoDelete: false,
			CreatedAt:    int64(7000 + i),
			UpdatedAt:    int64(7000 + i),
			Tasks:        []database.CreateTaskParams{},
		}
		if _, _, err := db.CreateTaskList(params); err != nil {
			t.Fatalf("CreateTaskList %s: %v", id, err)
		}
	}

	lists, _, err := db.ListTaskLists("")
	if err != nil {
		t.Fatalf("ListTaskLists: %v", err)
	}

	if len(lists) != 3 {
		t.Fatalf("expected 3 task lists, got %d", len(lists))
	}
}

func TestListTaskLists_FiltersByParentDir(t *testing.T) {
	db := newTestDB(t)

	dirs := []string{"", "Work", "Personal"}
	for i, dir := range dirs {
		params := database.CreateTaskListParams{
			Id:           "tl-filter-" + dir + "-001",
			Title:        "List in " + dir,
			ParentDir:    dir,
			IsAutoDelete: false,
			CreatedAt:    int64(8000 + i),
			UpdatedAt:    int64(8000 + i),
			Tasks:        []database.CreateTaskParams{},
		}
		if _, _, err := db.CreateTaskList(params); err != nil {
			t.Fatalf("CreateTaskList in %q: %v", dir, err)
		}
	}

	// Add a second item in "Work" to verify count.
	params := database.CreateTaskListParams{
		Id:           "tl-filter-Work-002",
		Title:        "Another Work List",
		ParentDir:    "Work",
		IsAutoDelete: false,
		CreatedAt:    8010,
		UpdatedAt:    8010,
		Tasks:        []database.CreateTaskParams{},
	}
	if _, _, err := db.CreateTaskList(params); err != nil {
		t.Fatalf("CreateTaskList second Work: %v", err)
	}

	// ListTaskLists("Work") should return only the Work ones.
	lists, _, err := db.ListTaskLists("Work")
	if err != nil {
		t.Fatalf("ListTaskLists(Work): %v", err)
	}

	if len(lists) != 2 {
		t.Fatalf("expected 2 task lists in Work, got %d", len(lists))
	}

	for _, tl := range lists {
		if tl.ParentDir != "Work" {
			t.Errorf("expected ParentDir %q, got %q", "Work", tl.ParentDir)
		}
	}
}

func TestListTaskListsWithCounts(t *testing.T) {
	db := newTestDB(t)

	// Create a task list with 3 main tasks: 2 done, 1 open.
	params := database.CreateTaskListParams{
		Id:           "tl-counts-001",
		Title:        "Counted List",
		ParentDir:    "",
		IsAutoDelete: false,
		CreatedAt:    9000,
		UpdatedAt:    9000,
		Tasks: []database.CreateTaskParams{
			{
				Id:          "count-task-001",
				Description: "Done task 1",
				IsDone:      true,
			},
			{
				Id:          "count-task-002",
				Description: "Done task 2",
				IsDone:      true,
			},
			{
				Id:          "count-task-003",
				Description: "Open task",
				IsDone:      false,
			},
		},
	}

	if _, _, err := db.CreateTaskList(params); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	lists, err := db.ListTaskListsWithCounts("")
	if err != nil {
		t.Fatalf("ListTaskListsWithCounts: %v", err)
	}

	if len(lists) != 1 {
		t.Fatalf("expected 1 task list, got %d", len(lists))
	}

	tl := lists[0]
	if tl.TotalTaskCount != 3 {
		t.Errorf("expected TotalTaskCount 3, got %d", tl.TotalTaskCount)
	}
	if tl.DoneTaskCount != 2 {
		t.Errorf("expected DoneTaskCount 2, got %d", tl.DoneTaskCount)
	}
}
