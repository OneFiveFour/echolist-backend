package file_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	"echolist-backend/database"
	"echolist-backend/file"
	"echolist-backend/notes"
	"echolist-backend/tasks"

	filev1 "echolist-backend/proto/gen/file/v1"
	notespb "echolist-backend/proto/gen/notes/v1"
	taskspb "echolist-backend/proto/gen/tasks/v1"
)

func TestListFiles_ReturnsFolders(t *testing.T) {
	dataDir := file.ResolvedTempDir(t)
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
	dataDir := file.ResolvedTempDir(t)
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
	dataDir := file.ResolvedTempDir(t)
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
	dataDir := file.ResolvedTempDir(t)
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
	dataDir := file.ResolvedTempDir(t)
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

func TestListFiles_TaskListUrgencyCounts(t *testing.T) {
	// Calendar arithmetic must also hold across year, leap-day and DST boundaries.
	for _, referenceDate := range []string{"2026-12-30", "2028-02-28", "2026-03-28", "2026-10-24"} {
		t.Run(referenceDate, func(t *testing.T) {
			db := file.NewTestDB(t)
			srv := file.NewFileServer(file.ResolvedTempDir(t), db, file.NopLogger())
			today, err := time.Parse(time.DateOnly, referenceDate)
			if err != nil {
				t.Fatal(err)
			}
			date := func(offset int) *string {
				value := today.AddDate(0, 0, offset).Format(time.DateOnly)
				return &value
			}
			emptyDate := ""
			recurrence := "FREQ=DAILY"
			_, _, err = db.CreateTaskList(database.CreateTaskListParams{
				Id: "counted", Title: "Counted", Tasks: []database.CreateTaskParams{
					{Id: "yesterday", DueDate: date(-1)},
					{Id: "today", DueDate: date(0)},
					{Id: "tomorrow", DueDate: date(1)},
					{Id: "two-days", DueDate: date(2)},
					{Id: "three-days", DueDate: date(3), Recurrence: &emptyDate},
					{Id: "four-days", DueDate: date(4)},
					{Id: "done-overdue", IsDone: true, DueDate: date(-2)},
					{Id: "done-soon", IsDone: true, DueDate: date(1)},
					{Id: "no-date"},
					{Id: "empty-date", DueDate: &emptyDate},
					{Id: "recurring", DueDate: date(2), Recurrence: &recurrence, SubTasks: []database.CreateTaskParams{
						{Id: "overdue-subtask", DueDate: date(-1)},
						{Id: "soon-subtask", DueDate: date(1)},
					}},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, list := range []database.CreateTaskListParams{
				{Id: "empty", Title: "Empty"},
				{Id: "nested", Title: "Nested", ParentDir: "Work", Tasks: []database.CreateTaskParams{{Id: "nested-task", DueDate: date(-1)}}},
			} {
				if _, _, err := db.CreateTaskList(list); err != nil {
					t.Fatal(err)
				}
			}
			for _, check := range []struct {
				date          *string
				overdue, soon int32
			}{
				{nil, 0, 0},
				{&referenceDate, 2, 4},
				{date(1), 3, 4},
			} {
				resp, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{ReferenceDate: check.date})
				if err != nil {
					t.Fatal(err)
				}
				if len(resp.Entries) != 2 {
					t.Fatalf("expected two root lists, got %d", len(resp.Entries))
				}
				for _, entry := range resp.Entries {
					meta := entry.GetTaskListMetadata()
					if meta == nil {
						t.Fatal("missing metadata")
					}
					if check.date == nil {
						if meta.OverdueTaskCount != nil || meta.DueSoonTaskCount != nil {
							t.Fatal("urgency counts must be absent without a reference date")
						}
						continue
					}
					if meta.OverdueTaskCount == nil || meta.DueSoonTaskCount == nil {
						t.Fatal("urgency counts must be present, including zero")
					}
					overdue, soon := check.overdue, check.soon
					if meta.Id == "empty" {
						overdue, soon = 0, 0
					} else if meta.TotalTaskCount != 11 || meta.DoneTaskCount != 2 {
						t.Fatalf("unexpected total/done counts: %v", meta)
					}
					if meta.GetOverdueTaskCount() != overdue || meta.GetDueSoonTaskCount() != soon {
						t.Fatalf("got urgency %d/%d, want %d/%d", meta.GetOverdueTaskCount(), meta.GetDueSoonTaskCount(), overdue, soon)
					}
					// Optional zero values must survive the JSON transport too.
					wire, err := protojson.Marshal(meta)
					if err != nil {
						t.Fatal(err)
					}
					var decoded filev1.TaskListMetadata
					if err := protojson.Unmarshal(wire, &decoded); err != nil {
						t.Fatal(err)
					}
					if decoded.OverdueTaskCount == nil || decoded.DueSoonTaskCount == nil {
						t.Fatalf("lost presence over JSON: %s", wire)
					}
				}
			}
		})
	}
}

func TestListFiles_InvalidReferenceDate(t *testing.T) {
	srv := file.NewFileServer(file.ResolvedTempDir(t), file.NewTestDB(t), file.NopLogger())
	for _, value := range []string{"", "2026-02-29", "2026-11-31", "2026-1-01", "2026-11-21T00:00:00Z", "garbage"} {
		t.Run(value, func(t *testing.T) {
			_, err := srv.ListFiles(context.Background(), &filev1.ListFilesRequest{ReferenceDate: &value})
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("expected invalid argument, got %v", err)
			}
		})
	}
}

func TestListFiles_NotePathFormat(t *testing.T) {
	dataDir := file.ResolvedTempDir(t)
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
	dataDir := file.ResolvedTempDir(t)
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
	dataDir := file.ResolvedTempDir(t)
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
	dataDir := file.ResolvedTempDir(t)
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
	dataDir := file.ResolvedTempDir(t)
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
