package file_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/file"
	"echolist-backend/notes"
	"echolist-backend/tasks"

	filev1 "echolist-backend/proto/gen/file/v1"
	notespb "echolist-backend/proto/gen/notes/v1"
	taskspb "echolist-backend/proto/gen/tasks/v1"
)

func TestCreateFolder_Success(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	_, err := fileSrv.CreateFolder(ctx, &filev1.CreateFolderRequest{
		ParentDir: "",
		Name:      "MyFolder",
	})
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	var found bool
	for _, entry := range resp.Entries {
		if entry.ItemType == filev1.ItemType_ITEM_TYPE_FOLDER && entry.Title == "MyFolder" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected MyFolder to appear in ListFiles as a FOLDER entry")
	}
}

func TestDeleteFolder_CascadesToNotes(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()
	taskSrv := tasks.NewTaskServer(dataDir, db, logger)
	noteSrv := notes.NewNotesServer(dataDir, db, logger)
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	// Create folder "Projects"
	_, err := fileSrv.CreateFolder(ctx, &filev1.CreateFolderRequest{
		ParentDir: "",
		Name:      "Projects",
	})
	if err != nil {
		t.Fatalf("CreateFolder failed: %v", err)
	}

	// Create a note in "Projects"
	noteResp, err := noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
		ParentDir: "Projects",
		Title:     "ProjectNote",
		Content:   "Some project content",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}
	noteID := noteResp.Note.Id

	// Create a task list in "Projects"
	tlResp, err := taskSrv.CreateTaskList(ctx, &taskspb.CreateTaskListRequest{
		Title:     "ProjectTasks",
		ParentDir: "Projects",
		Tasks: []*taskspb.MainTask{
			{Description: "Task 1", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}
	tlID := tlResp.TaskList.Id

	// Delete folder "Projects"
	_, err = fileSrv.DeleteFolder(ctx, &filev1.DeleteFolderRequest{
		FolderPath: "Projects",
	})
	if err != nil {
		t.Fatalf("DeleteFolder failed: %v", err)
	}

	// Verify: folder removed from disk
	folderPath := filepath.Join(dataDir, "Projects")
	if _, err := os.Stat(folderPath); !os.IsNotExist(err) {
		t.Fatalf("expected folder to be removed from disk, got err: %v", err)
	}

	// Verify: GetNote returns NotFound
	_, err = noteSrv.GetNote(ctx, &notespb.GetNoteRequest{Id: noteID})
	if err == nil {
		t.Fatal("expected GetNote to return error after folder deletion")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound for GetNote, got %v", connect.CodeOf(err))
	}

	// Verify: GetTaskList returns NotFound
	_, err = taskSrv.GetTaskList(ctx, &taskspb.GetTaskListRequest{Id: tlID})
	if err == nil {
		t.Fatal("expected GetTaskList to return error after folder deletion")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound for GetTaskList, got %v", connect.CodeOf(err))
	}
}

func TestDeleteFolder_NestedSubfolders(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()
	taskSrv := tasks.NewTaskServer(dataDir, db, logger)
	noteSrv := notes.NewNotesServer(dataDir, db, logger)
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	// Create folder "Work"
	_, err := fileSrv.CreateFolder(ctx, &filev1.CreateFolderRequest{
		ParentDir: "",
		Name:      "Work",
	})
	if err != nil {
		t.Fatalf("CreateFolder Work failed: %v", err)
	}

	// Create subfolder "Work/2026" on disk
	if err := os.MkdirAll(filepath.Join(dataDir, "Work", "2026"), 0o755); err != nil {
		t.Fatalf("failed to create Work/2026: %v", err)
	}

	// Create a note in "Work"
	noteResp1, err := noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
		ParentDir: "Work",
		Title:     "WorkNote",
		Content:   "Work content",
	})
	if err != nil {
		t.Fatalf("CreateNote in Work failed: %v", err)
	}
	noteID1 := noteResp1.Note.Id

	// Create a note in "Work/2026"
	noteResp2, err := noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
		ParentDir: "Work/2026",
		Title:     "YearNote",
		Content:   "2026 content",
	})
	if err != nil {
		t.Fatalf("CreateNote in Work/2026 failed: %v", err)
	}
	noteID2 := noteResp2.Note.Id

	// Create a task list in "Work/2026"
	tlResp, err := taskSrv.CreateTaskList(ctx, &taskspb.CreateTaskListRequest{
		Title:     "YearTasks",
		ParentDir: "Work/2026",
		Tasks: []*taskspb.MainTask{
			{Description: "Plan", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList in Work/2026 failed: %v", err)
	}
	tlID := tlResp.TaskList.Id

	// Delete folder "Work"
	_, err = fileSrv.DeleteFolder(ctx, &filev1.DeleteFolderRequest{
		FolderPath: "Work",
	})
	if err != nil {
		t.Fatalf("DeleteFolder Work failed: %v", err)
	}

	// Verify: all notes and task lists return NotFound
	_, err = noteSrv.GetNote(ctx, &notespb.GetNoteRequest{Id: noteID1})
	if err == nil || connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected NotFound for note in Work, got: %v", err)
	}

	_, err = noteSrv.GetNote(ctx, &notespb.GetNoteRequest{Id: noteID2})
	if err == nil || connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected NotFound for note in Work/2026, got: %v", err)
	}

	_, err = taskSrv.GetTaskList(ctx, &taskspb.GetTaskListRequest{Id: tlID})
	if err == nil || connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected NotFound for task list in Work/2026, got: %v", err)
	}
}

func TestUpdateFolder_RenameUpdatesParentDir(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()
	taskSrv := tasks.NewTaskServer(dataDir, db, logger)
	noteSrv := notes.NewNotesServer(dataDir, db, logger)
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	// Create folder "OldName"
	_, err := fileSrv.CreateFolder(ctx, &filev1.CreateFolderRequest{
		ParentDir: "",
		Name:      "OldName",
	})
	if err != nil {
		t.Fatalf("CreateFolder OldName failed: %v", err)
	}

	// Create a note in "OldName"
	_, err = noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
		ParentDir: "OldName",
		Title:     "RenameNote",
		Content:   "rename content",
	})
	if err != nil {
		t.Fatalf("CreateNote in OldName failed: %v", err)
	}

	// Create a task list in "OldName"
	_, err = taskSrv.CreateTaskList(ctx, &taskspb.CreateTaskListRequest{
		Title:     "RenameTasks",
		ParentDir: "OldName",
		Tasks: []*taskspb.MainTask{
			{Description: "Do something", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList in OldName failed: %v", err)
	}

	// Rename folder
	_, err = fileSrv.UpdateFolder(ctx, &filev1.UpdateFolderRequest{
		FolderPath: "OldName",
		NewName:    "NewName",
	})
	if err != nil {
		t.Fatalf("UpdateFolder failed: %v", err)
	}

	// Verify: ListNotes("NewName") returns the note
	notesResp, err := noteSrv.ListNotes(ctx, &notespb.ListNotesRequest{ParentDir: "NewName"})
	if err != nil {
		t.Fatalf("ListNotes NewName failed: %v", err)
	}
	if len(notesResp.Notes) != 1 {
		t.Fatalf("expected 1 note in NewName, got %d", len(notesResp.Notes))
	}
	if notesResp.Notes[0].Title != "RenameNote" {
		t.Fatalf("expected note title 'RenameNote', got %q", notesResp.Notes[0].Title)
	}

	// Verify: ListNotes("OldName") returns empty (folder doesn't exist on disk anymore)
	// After rename, OldName dir doesn't exist, so ListNotes would fail with NotFound.
	// We verify by checking the directory doesn't exist.
	oldDirPath := filepath.Join(dataDir, "OldName")
	if _, err := os.Stat(oldDirPath); !os.IsNotExist(err) {
		t.Fatalf("expected OldName directory to not exist, got err: %v", err)
	}

	// Verify: ListTaskLists("NewName") returns the task list
	tlResp, err := taskSrv.ListTaskLists(ctx, &taskspb.ListTaskListsRequest{ParentDir: "NewName"})
	if err != nil {
		t.Fatalf("ListTaskLists NewName failed: %v", err)
	}
	if len(tlResp.TaskLists) != 1 {
		t.Fatalf("expected 1 task list in NewName, got %d", len(tlResp.TaskLists))
	}
	if tlResp.TaskLists[0].Title != "RenameTasks" {
		t.Fatalf("expected task list title 'RenameTasks', got %q", tlResp.TaskLists[0].Title)
	}

	// Verify: ListTaskLists("OldName") would fail since dir doesn't exist
	// Already verified above that OldName dir is gone
}

func TestUpdateFolder_NestedSubfoldersRename(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()
	noteSrv := notes.NewNotesServer(dataDir, db, logger)
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	// Create folder "Docs"
	_, err := fileSrv.CreateFolder(ctx, &filev1.CreateFolderRequest{
		ParentDir: "",
		Name:      "Docs",
	})
	if err != nil {
		t.Fatalf("CreateFolder Docs failed: %v", err)
	}

	// Create subfolder "Docs/Reports" on disk
	if err := os.MkdirAll(filepath.Join(dataDir, "Docs", "Reports"), 0o755); err != nil {
		t.Fatalf("failed to create Docs/Reports: %v", err)
	}

	// Create a note in "Docs"
	_, err = noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
		ParentDir: "Docs",
		Title:     "DocsNote",
		Content:   "docs content",
	})
	if err != nil {
		t.Fatalf("CreateNote in Docs failed: %v", err)
	}

	// Create a note in "Docs/Reports"
	_, err = noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
		ParentDir: "Docs/Reports",
		Title:     "ReportNote",
		Content:   "report content",
	})
	if err != nil {
		t.Fatalf("CreateNote in Docs/Reports failed: %v", err)
	}

	// Rename folder "Docs" -> "Documents"
	_, err = fileSrv.UpdateFolder(ctx, &filev1.UpdateFolderRequest{
		FolderPath: "Docs",
		NewName:    "Documents",
	})
	if err != nil {
		t.Fatalf("UpdateFolder Docs->Documents failed: %v", err)
	}

	// Verify: ListNotes("Documents") returns the note that was in "Docs"
	notesResp, err := noteSrv.ListNotes(ctx, &notespb.ListNotesRequest{ParentDir: "Documents"})
	if err != nil {
		t.Fatalf("ListNotes Documents failed: %v", err)
	}
	if len(notesResp.Notes) != 1 {
		t.Fatalf("expected 1 note in Documents, got %d", len(notesResp.Notes))
	}
	if notesResp.Notes[0].Title != "DocsNote" {
		t.Fatalf("expected note title 'DocsNote', got %q", notesResp.Notes[0].Title)
	}

	// Verify: ListNotes("Documents/Reports") returns the note that was in "Docs/Reports"
	notesResp2, err := noteSrv.ListNotes(ctx, &notespb.ListNotesRequest{ParentDir: "Documents/Reports"})
	if err != nil {
		t.Fatalf("ListNotes Documents/Reports failed: %v", err)
	}
	if len(notesResp2.Notes) != 1 {
		t.Fatalf("expected 1 note in Documents/Reports, got %d", len(notesResp2.Notes))
	}
	if notesResp2.Notes[0].Title != "ReportNote" {
		t.Fatalf("expected note title 'ReportNote', got %q", notesResp2.Notes[0].Title)
	}

	// Verify: ListNotes("Docs") - directory doesn't exist anymore
	oldDocsPath := filepath.Join(dataDir, "Docs")
	if _, err := os.Stat(oldDocsPath); !os.IsNotExist(err) {
		t.Fatalf("expected Docs directory to not exist, got err: %v", err)
	}

	// Verify: ListNotes("Docs/Reports") - directory doesn't exist anymore
	oldReportsPath := filepath.Join(dataDir, "Docs", "Reports")
	if _, err := os.Stat(oldReportsPath); !os.IsNotExist(err) {
		t.Fatalf("expected Docs/Reports directory to not exist, got err: %v", err)
	}
}

func TestDeleteFolder_NonExistentReturnsNotFound(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	_, err := fileSrv.DeleteFolder(ctx, &filev1.DeleteFolderRequest{
		FolderPath: "NonExistent",
	})
	if err == nil {
		t.Fatal("expected error for non-existent folder")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}

func TestUpdateFolder_NonExistentReturnsNotFound(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	_, err := fileSrv.UpdateFolder(ctx, &filev1.UpdateFolderRequest{
		FolderPath: "NonExistent",
		NewName:    "Something",
	})
	if err == nil {
		t.Fatal("expected error for non-existent folder")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}
}
