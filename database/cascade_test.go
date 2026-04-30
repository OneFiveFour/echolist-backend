package database_test

import (
	"testing"

	"echolist-backend/database"
)

func TestDeleteTaskList_CascadesToTasks(t *testing.T) {
	db := newTestDB(t)

	// Create a task list with 2 main tasks, one having subtasks.
	params := database.CreateTaskListParams{
		Id:           "tl-cascade-001",
		Title:        "Cascade Test",
		ParentDir:    "CascadeDir",
		IsAutoDelete: false,
		CreatedAt:    1000,
		UpdatedAt:    1000,
		Tasks: []database.CreateTaskParams{
			{
				Id:          "cascade-task-001",
				Description: "Main task with subtasks",
				IsDone:      false,
				SubTasks: []database.CreateTaskParams{
					{
						Id:          "cascade-sub-001",
						Description: "Subtask 1",
						IsDone:      false,
					},
					{
						Id:          "cascade-sub-002",
						Description: "Subtask 2",
						IsDone:      true,
					},
				},
			},
			{
				Id:          "cascade-task-002",
				Description: "Simple main task",
				IsDone:      false,
			},
		},
	}

	_, _, err := db.CreateTaskList(params)
	if err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	// Delete the task list.
	deleted, err := db.DeleteTaskList("tl-cascade-001")
	if err != nil {
		t.Fatalf("DeleteTaskList: %v", err)
	}
	if !deleted {
		t.Fatal("expected DeleteTaskList to return true")
	}

	// Verify GetTaskList returns ErrNotFound.
	_, _, err = db.GetTaskList("tl-cascade-001")
	if err != database.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got: %v", err)
	}

	// Verify tasks are gone by creating a new task list and confirming no stale data.
	newParams := database.CreateTaskListParams{
		Id:           "tl-cascade-002",
		Title:        "New List",
		ParentDir:    "CascadeDir",
		IsAutoDelete: false,
		CreatedAt:    2000,
		UpdatedAt:    2000,
		Tasks:        []database.CreateTaskParams{},
	}

	_, newTasks, err := db.CreateTaskList(newParams)
	if err != nil {
		t.Fatalf("CreateTaskList (new): %v", err)
	}
	if len(newTasks) != 0 {
		t.Errorf("expected 0 tasks in new list, got %d", len(newTasks))
	}

	// Verify the old task list's tasks don't appear anywhere.
	_, tasks, err := db.GetTaskList("tl-cascade-002")
	if err != nil {
		t.Fatalf("GetTaskList (new): %v", err)
	}
	for _, task := range tasks {
		if task.Id == "cascade-task-001" || task.Id == "cascade-task-002" ||
			task.Id == "cascade-sub-001" || task.Id == "cascade-sub-002" {
			t.Errorf("stale task %q still exists after cascade delete", task.Id)
		}
	}
}

func TestDeleteByParentDir(t *testing.T) {
	db := newTestDB(t)

	// Insert 2 notes with parent_dir="Projects".
	note1 := database.InsertNoteParams{
		Id:        "note-dir-001",
		Title:     "Project Note 1",
		ParentDir: "Projects",
		Preview:   "Preview 1",
		CreatedAt: 1000,
		UpdatedAt: 1000,
	}
	note2 := database.InsertNoteParams{
		Id:        "note-dir-002",
		Title:     "Project Note 2",
		ParentDir: "Projects",
		Preview:   "Preview 2",
		CreatedAt: 1001,
		UpdatedAt: 1001,
	}
	if err := db.InsertNote(note1); err != nil {
		t.Fatalf("InsertNote 1: %v", err)
	}
	if err := db.InsertNote(note2); err != nil {
		t.Fatalf("InsertNote 2: %v", err)
	}

	// Create 1 task list with parent_dir="Projects".
	tlParams := database.CreateTaskListParams{
		Id:           "tl-dir-001",
		Title:        "Project Tasks",
		ParentDir:    "Projects",
		IsAutoDelete: false,
		CreatedAt:    1002,
		UpdatedAt:    1002,
		Tasks: []database.CreateTaskParams{
			{
				Id:          "dir-task-001",
				Description: "A task",
				IsDone:      false,
			},
		},
	}
	if _, _, err := db.CreateTaskList(tlParams); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	// Delete by parent dir.
	if err := db.DeleteByParentDir("Projects"); err != nil {
		t.Fatalf("DeleteByParentDir: %v", err)
	}

	// Verify GetNote returns ErrNotFound for both notes.
	_, err := db.GetNote("note-dir-001")
	if err != database.ErrNotFound {
		t.Errorf("expected ErrNotFound for note-dir-001, got: %v", err)
	}
	_, err = db.GetNote("note-dir-002")
	if err != database.ErrNotFound {
		t.Errorf("expected ErrNotFound for note-dir-002, got: %v", err)
	}

	// Verify GetTaskList returns ErrNotFound for the task list.
	_, _, err = db.GetTaskList("tl-dir-001")
	if err != database.ErrNotFound {
		t.Errorf("expected ErrNotFound for tl-dir-001, got: %v", err)
	}
}

func TestDeleteByParentDir_NestedPaths(t *testing.T) {
	db := newTestDB(t)

	// Insert notes in nested directories under "Work".
	notes := []database.InsertNoteParams{
		{Id: "note-nested-001", Title: "Work Note", ParentDir: "Work", Preview: "p1", CreatedAt: 1000, UpdatedAt: 1000},
		{Id: "note-nested-002", Title: "2026 Note", ParentDir: "Work/2026", Preview: "p2", CreatedAt: 1001, UpdatedAt: 1001},
		{Id: "note-nested-003", Title: "Q1 Note", ParentDir: "Work/2026/Q1", Preview: "p3", CreatedAt: 1002, UpdatedAt: 1002},
	}
	for _, n := range notes {
		if err := db.InsertNote(n); err != nil {
			t.Fatalf("InsertNote %s: %v", n.Id, err)
		}
	}

	// Create task list in nested directory.
	tlParams := database.CreateTaskListParams{
		Id:           "tl-nested-001",
		Title:        "Nested Tasks",
		ParentDir:    "Work/2026",
		IsAutoDelete: false,
		CreatedAt:    1003,
		UpdatedAt:    1003,
		Tasks:        []database.CreateTaskParams{},
	}
	if _, _, err := db.CreateTaskList(tlParams); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	// Insert a note in "Personal" that should NOT be deleted.
	personalNote := database.InsertNoteParams{
		Id:        "note-personal-001",
		Title:     "Personal Note",
		ParentDir: "Personal",
		Preview:   "personal",
		CreatedAt: 1004,
		UpdatedAt: 1004,
	}
	if err := db.InsertNote(personalNote); err != nil {
		t.Fatalf("InsertNote personal: %v", err)
	}

	// Delete by parent dir "Work".
	if err := db.DeleteByParentDir("Work"); err != nil {
		t.Fatalf("DeleteByParentDir: %v", err)
	}

	// Verify ALL items under "Work" are deleted.
	for _, id := range []string{"note-nested-001", "note-nested-002", "note-nested-003"} {
		_, err := db.GetNote(id)
		if err != database.ErrNotFound {
			t.Errorf("expected ErrNotFound for %s, got: %v", id, err)
		}
	}

	_, _, err := db.GetTaskList("tl-nested-001")
	if err != database.ErrNotFound {
		t.Errorf("expected ErrNotFound for tl-nested-001, got: %v", err)
	}

	// Verify "Personal" note is NOT deleted.
	n, err := db.GetNote("note-personal-001")
	if err != nil {
		t.Fatalf("expected Personal note to still exist, got error: %v", err)
	}
	if n.Title != "Personal Note" {
		t.Errorf("expected title %q, got %q", "Personal Note", n.Title)
	}
}

func TestRenameParentDir(t *testing.T) {
	db := newTestDB(t)

	// Insert 2 notes with parent_dir="OldFolder".
	note1 := database.InsertNoteParams{
		Id:        "note-rename-001",
		Title:     "Note A",
		ParentDir: "OldFolder",
		Preview:   "preview a",
		CreatedAt: 1000,
		UpdatedAt: 1000,
	}
	note2 := database.InsertNoteParams{
		Id:        "note-rename-002",
		Title:     "Note B",
		ParentDir: "OldFolder",
		Preview:   "preview b",
		CreatedAt: 1001,
		UpdatedAt: 1001,
	}
	if err := db.InsertNote(note1); err != nil {
		t.Fatalf("InsertNote 1: %v", err)
	}
	if err := db.InsertNote(note2); err != nil {
		t.Fatalf("InsertNote 2: %v", err)
	}

	// Create 1 task list with parent_dir="OldFolder".
	tlParams := database.CreateTaskListParams{
		Id:           "tl-rename-001",
		Title:        "Rename Tasks",
		ParentDir:    "OldFolder",
		IsAutoDelete: false,
		CreatedAt:    1002,
		UpdatedAt:    1002,
		Tasks:        []database.CreateTaskParams{},
	}
	if _, _, err := db.CreateTaskList(tlParams); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	// Rename parent dir.
	if err := db.RenameParentDir("OldFolder", "NewFolder"); err != nil {
		t.Fatalf("RenameParentDir: %v", err)
	}

	// Verify ListNotes("NewFolder") returns the 2 notes.
	newNotes, err := db.ListNotes("NewFolder")
	if err != nil {
		t.Fatalf("ListNotes(NewFolder): %v", err)
	}
	if len(newNotes) != 2 {
		t.Errorf("expected 2 notes in NewFolder, got %d", len(newNotes))
	}

	// Verify ListNotes("OldFolder") returns empty.
	oldNotes, err := db.ListNotes("OldFolder")
	if err != nil {
		t.Fatalf("ListNotes(OldFolder): %v", err)
	}
	if len(oldNotes) != 0 {
		t.Errorf("expected 0 notes in OldFolder, got %d", len(oldNotes))
	}

	// Verify ListTaskLists("NewFolder") returns the task list.
	newLists, _, err := db.ListTaskLists("NewFolder")
	if err != nil {
		t.Fatalf("ListTaskLists(NewFolder): %v", err)
	}
	if len(newLists) != 1 {
		t.Errorf("expected 1 task list in NewFolder, got %d", len(newLists))
	}

	// Verify ListTaskLists("OldFolder") returns empty.
	oldLists, _, err := db.ListTaskLists("OldFolder")
	if err != nil {
		t.Fatalf("ListTaskLists(OldFolder): %v", err)
	}
	if len(oldLists) != 0 {
		t.Errorf("expected 0 task lists in OldFolder, got %d", len(oldLists))
	}
}

func TestRenameParentDir_NestedPaths(t *testing.T) {
	db := newTestDB(t)

	// Insert notes in nested directories under "Docs".
	notes := []database.InsertNoteParams{
		{Id: "note-rn-001", Title: "Docs Note", ParentDir: "Docs", Preview: "p1", CreatedAt: 1000, UpdatedAt: 1000},
		{Id: "note-rn-002", Title: "2026 Note", ParentDir: "Docs/2026", Preview: "p2", CreatedAt: 1001, UpdatedAt: 1001},
		{Id: "note-rn-003", Title: "Reports Note", ParentDir: "Docs/2026/Reports", Preview: "p3", CreatedAt: 1002, UpdatedAt: 1002},
	}
	for _, n := range notes {
		if err := db.InsertNote(n); err != nil {
			t.Fatalf("InsertNote %s: %v", n.Id, err)
		}
	}

	// Create task list in nested directory.
	tlParams := database.CreateTaskListParams{
		Id:           "tl-rn-001",
		Title:        "Nested Tasks",
		ParentDir:    "Docs/2026",
		IsAutoDelete: false,
		CreatedAt:    1003,
		UpdatedAt:    1003,
		Tasks:        []database.CreateTaskParams{},
	}
	if _, _, err := db.CreateTaskList(tlParams); err != nil {
		t.Fatalf("CreateTaskList: %v", err)
	}

	// Rename "Docs" to "Documents".
	if err := db.RenameParentDir("Docs", "Documents"); err != nil {
		t.Fatalf("RenameParentDir: %v", err)
	}

	// Verify ListNotes("Documents") returns the note that was in "Docs".
	docNotes, err := db.ListNotes("Documents")
	if err != nil {
		t.Fatalf("ListNotes(Documents): %v", err)
	}
	if len(docNotes) != 1 {
		t.Errorf("expected 1 note in Documents, got %d", len(docNotes))
	}

	// Verify ListNotes("Documents/2026") returns the note that was in "Docs/2026".
	doc2026Notes, err := db.ListNotes("Documents/2026")
	if err != nil {
		t.Fatalf("ListNotes(Documents/2026): %v", err)
	}
	if len(doc2026Notes) != 1 {
		t.Errorf("expected 1 note in Documents/2026, got %d", len(doc2026Notes))
	}

	// Verify ListNotes("Documents/2026/Reports") returns the note that was in "Docs/2026/Reports".
	docReportsNotes, err := db.ListNotes("Documents/2026/Reports")
	if err != nil {
		t.Fatalf("ListNotes(Documents/2026/Reports): %v", err)
	}
	if len(docReportsNotes) != 1 {
		t.Errorf("expected 1 note in Documents/2026/Reports, got %d", len(docReportsNotes))
	}

	// Verify ListTaskLists("Documents/2026") returns the task list.
	docLists, _, err := db.ListTaskLists("Documents/2026")
	if err != nil {
		t.Fatalf("ListTaskLists(Documents/2026): %v", err)
	}
	if len(docLists) != 1 {
		t.Errorf("expected 1 task list in Documents/2026, got %d", len(docLists))
	}

	// Verify old paths return empty.
	oldDocsNotes, err := db.ListNotes("Docs")
	if err != nil {
		t.Fatalf("ListNotes(Docs): %v", err)
	}
	if len(oldDocsNotes) != 0 {
		t.Errorf("expected 0 notes in Docs, got %d", len(oldDocsNotes))
	}

	oldDocs2026Notes, err := db.ListNotes("Docs/2026")
	if err != nil {
		t.Fatalf("ListNotes(Docs/2026): %v", err)
	}
	if len(oldDocs2026Notes) != 0 {
		t.Errorf("expected 0 notes in Docs/2026, got %d", len(oldDocs2026Notes))
	}
}
