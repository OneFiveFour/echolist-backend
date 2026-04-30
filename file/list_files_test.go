package file_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"echolist-backend/file"
	"echolist-backend/notes"
	"echolist-backend/tasks"

	filev1 "echolist-backend/proto/gen/file/v1"
	notespb "echolist-backend/proto/gen/notes/v1"
	taskspb "echolist-backend/proto/gen/tasks/v1"
)

func TestListFiles_ReturnsFolders(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	// Create 2 subdirectories on disk
	if err := os.Mkdir(filepath.Join(dataDir, "FolderA"), 0o755); err != nil {
		t.Fatalf("failed to create FolderA: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dataDir, "FolderB"), 0o755); err != nil {
		t.Fatalf("failed to create FolderB: %v", err)
	}

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	folderNames := make(map[string]bool)
	for _, entry := range resp.Entries {
		if entry.ItemType == filev1.ItemType_ITEM_TYPE_FOLDER {
			folderNames[entry.Title] = true
		}
	}

	if !folderNames["FolderA"] {
		t.Fatal("expected FolderA in entries")
	}
	if !folderNames["FolderB"] {
		t.Fatal("expected FolderB in entries")
	}
}

func TestListFiles_ReturnsNotesFromSQLite(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	noteSrv := notes.NewNotesServer(dataDir, db, logger)
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	createResp, err := noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
		ParentDir: "",
		Title:     "MyNote",
		Content:   "Hello",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}
	noteID := createResp.Note.Id

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	var found *filev1.FileEntry
	for _, entry := range resp.Entries {
		if entry.ItemType == filev1.ItemType_ITEM_TYPE_NOTE {
			found = entry
			break
		}
	}
	if found == nil {
		t.Fatal("expected a note entry in ListFiles")
	}
	if found.Title != "MyNote" {
		t.Fatalf("expected title 'MyNote', got %q", found.Title)
	}
	// Path should contain the note ID in <title>_<id>.md format
	if !strings.Contains(found.Path, noteID) {
		t.Fatalf("expected path to contain note ID %q, got %q", noteID, found.Path)
	}
	expectedSuffix := "_" + noteID + ".md"
	if !strings.HasSuffix(found.Path, expectedSuffix) {
		t.Fatalf("expected path to end with %q, got %q", expectedSuffix, found.Path)
	}
}

func TestListFiles_ReturnsTaskListsFromSQLite(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	taskSrv := tasks.NewTaskServer(dataDir, db, logger)
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	_, err := taskSrv.CreateTaskList(ctx, &taskspb.CreateTaskListRequest{
		Title:     "Shopping",
		ParentDir: "",
		Tasks: []*taskspb.MainTask{
			{Description: "Buy milk", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	var found *filev1.FileEntry
	for _, entry := range resp.Entries {
		if entry.ItemType == filev1.ItemType_ITEM_TYPE_TASK_LIST {
			found = entry
			break
		}
	}
	if found == nil {
		t.Fatal("expected a task list entry in ListFiles")
	}
	if found.Title != "Shopping" {
		t.Fatalf("expected title 'Shopping', got %q", found.Title)
	}
}

func TestListFiles_NoteEntryIncludesPreview(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	noteSrv := notes.NewNotesServer(dataDir, db, logger)
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	_, err := noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
		ParentDir: "",
		Title:     "PreviewNote",
		Content:   "Hello World Preview Test",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	var found *filev1.FileEntry
	for _, entry := range resp.Entries {
		if entry.ItemType == filev1.ItemType_ITEM_TYPE_NOTE {
			found = entry
			break
		}
	}
	if found == nil {
		t.Fatal("expected a note entry in ListFiles")
	}

	noteMeta := found.GetNoteMetadata()
	if noteMeta == nil {
		t.Fatal("expected NoteMetadata to be set")
	}
	if noteMeta.Preview != "Hello World Preview Test" {
		t.Fatalf("expected preview 'Hello World Preview Test', got %q", noteMeta.Preview)
	}
}

func TestListFiles_TaskListEntryIncludesCounts(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	taskSrv := tasks.NewTaskServer(dataDir, db, logger)
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	_, err := taskSrv.CreateTaskList(ctx, &taskspb.CreateTaskListRequest{
		Title:     "CountTest",
		ParentDir: "",
		Tasks: []*taskspb.MainTask{
			{Description: "Task 1", IsDone: true},
			{Description: "Task 2", IsDone: true},
			{Description: "Task 3", IsDone: false},
		},
		IsAutoDelete: false,
	})
	if err != nil {
		t.Fatalf("CreateTaskList failed: %v", err)
	}

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	var found *filev1.FileEntry
	for _, entry := range resp.Entries {
		if entry.ItemType == filev1.ItemType_ITEM_TYPE_TASK_LIST {
			found = entry
			break
		}
	}
	if found == nil {
		t.Fatal("expected a task list entry in ListFiles")
	}

	tlMeta := found.GetTaskListMetadata()
	if tlMeta == nil {
		t.Fatal("expected TaskListMetadata to be set")
	}
	if tlMeta.TotalTaskCount != 3 {
		t.Fatalf("expected TotalTaskCount == 3, got %d", tlMeta.TotalTaskCount)
	}
	if tlMeta.DoneTaskCount != 2 {
		t.Fatalf("expected DoneTaskCount == 2, got %d", tlMeta.DoneTaskCount)
	}
}

func TestListFiles_NotePathFormat(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	noteSrv := notes.NewNotesServer(dataDir, db, logger)
	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	createResp, err := noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
		ParentDir: "",
		Title:     "PathTest",
		Content:   "content",
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}
	noteID := createResp.Note.Id

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	var found *filev1.FileEntry
	for _, entry := range resp.Entries {
		if entry.ItemType == filev1.ItemType_ITEM_TYPE_NOTE {
			found = entry
			break
		}
	}
	if found == nil {
		t.Fatal("expected a note entry in ListFiles")
	}

	// Path should end with _<id>.md
	expectedSuffix := "_" + noteID + ".md"
	if !strings.HasSuffix(found.Path, expectedSuffix) {
		t.Fatalf("expected path to end with %q, got %q", expectedSuffix, found.Path)
	}
	// Path should start with the note's title
	if !strings.HasPrefix(found.Path, "PathTest") {
		t.Fatalf("expected path to start with 'PathTest', got %q", found.Path)
	}
}

func TestListFiles_OrphanNoteFileNotShown(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	// Create an orphan note file on disk (old format, no DB row)
	orphanPath := filepath.Join(dataDir, "note_hello.md")
	if err := os.WriteFile(orphanPath, []byte("orphan content"), 0o644); err != nil {
		t.Fatalf("failed to write orphan file: %v", err)
	}

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	for _, entry := range resp.Entries {
		if strings.Contains(entry.Path, "note_hello") || strings.Contains(entry.Title, "note_hello") {
			t.Fatalf("orphan note file should NOT appear in entries, but found: %+v", entry)
		}
	}
}

func TestListFiles_OrphanTaskListFileNotShown(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	// Create an orphan task list file on disk (old format, no DB row)
	orphanPath := filepath.Join(dataDir, "tasks_todo.md")
	if err := os.WriteFile(orphanPath, []byte("orphan tasks"), 0o644); err != nil {
		t.Fatalf("failed to write orphan file: %v", err)
	}

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	for _, entry := range resp.Entries {
		if strings.Contains(entry.Path, "tasks_todo") || strings.Contains(entry.Title, "tasks_todo") {
			t.Fatalf("orphan task list file should NOT appear in entries, but found: %+v", entry)
		}
	}
}

func TestListFiles_EmptyDirectory(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	// Create an empty subdirectory
	subDir := "emptydir"
	if err := os.Mkdir(filepath.Join(dataDir, subDir), 0o755); err != nil {
		t.Fatalf("failed to create empty subdir: %v", err)
	}

	resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: subDir})
	if err != nil {
		t.Fatalf("ListFiles failed: %v", err)
	}

	if len(resp.Entries) != 0 {
		t.Fatalf("expected 0 entries for empty directory, got %d", len(resp.Entries))
	}
}

func TestListFiles_RootSlashRejected(t *testing.T) {
	dataDir := t.TempDir()
	db := file.NewTestDB(t)
	logger := file.NopLogger()

	fileSrv := file.NewFileServer(dataDir, db, logger)
	ctx := context.Background()

	_, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: "/"})
	if err == nil {
		t.Fatal("expected error for ParentDir='/', got nil")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}
