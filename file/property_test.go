package file_test

import (
	"context"
	"fmt"
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

	"pgregory.net/rapid"
)

// --- Generators ---

// folderNameGen generates valid folder names.
func folderNameGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9_-]{0,19}`)
}

// noteContentGen generates valid note content.
func noteContentGen() *rapid.Generator[string] {
	return rapid.StringMatching(`[A-Za-z0-9 ]{1,200}`)
}

// --- Property Tests ---

// Feature: test-suite-sqlite-rewrite, Property 11: Folder Cascade Delete Removes All DB Rows
// Validates: Requirements 6.1, 6.2, 6.3, 6.4, 14.3
func TestProperty_FolderCascadeDeleteRemovesAllDBRows(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		db := file.NewTestDB(t)
		logger := file.NopLogger()
		taskSrv := tasks.NewTaskServer(dataDir, db, logger)
		noteSrv := notes.NewNotesServer(dataDir, db, logger)
		fileSrv := file.NewFileServer(dataDir, db, logger)
		ctx := context.Background()

		// Generate a folder name and create it
		folderName := folderNameGen().Draw(rt, "folderName")

		_, err := fileSrv.CreateFolder(ctx, &filev1.CreateFolderRequest{
			ParentDir: "",
			Name:      folderName,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}

		// Create 1-3 notes in that folder
		numNotes := rapid.IntRange(1, 3).Draw(rt, "numNotes")
		noteIDs := make([]string, 0, numNotes)
		for i := 0; i < numNotes; i++ {
			title := folderNameGen().Draw(rt, fmt.Sprintf("noteTitle-%d", i))
			content := noteContentGen().Draw(rt, fmt.Sprintf("noteContent-%d", i))
			resp, err := noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
				ParentDir: folderName,
				Title:     title,
				Content:   content,
			})
			if err != nil {
				rt.Fatalf("CreateNote[%d] failed: %v", i, err)
			}
			noteIDs = append(noteIDs, resp.Note.Id)
		}

		// Create 1-2 task lists in that folder
		numTaskLists := rapid.IntRange(1, 2).Draw(rt, "numTaskLists")
		taskListIDs := make([]string, 0, numTaskLists)
		for i := 0; i < numTaskLists; i++ {
			title := folderNameGen().Draw(rt, fmt.Sprintf("taskListTitle-%d", i))
			resp, err := taskSrv.CreateTaskList(ctx, &taskspb.CreateTaskListRequest{
				Title:     title,
				ParentDir: folderName,
				Tasks: []*taskspb.MainTask{
					{Description: "Task", IsDone: false},
				},
				IsAutoDelete: false,
			})
			if err != nil {
				rt.Fatalf("CreateTaskList[%d] failed: %v", i, err)
			}
			taskListIDs = append(taskListIDs, resp.TaskList.Id)
		}

		// Delete the folder
		_, err = fileSrv.DeleteFolder(ctx, &filev1.DeleteFolderRequest{
			FolderPath: folderName,
		})
		if err != nil {
			rt.Fatalf("DeleteFolder failed: %v", err)
		}

		// Verify: all notes return NotFound via GetNote
		for _, id := range noteIDs {
			_, err := noteSrv.GetNote(ctx, &notespb.GetNoteRequest{Id: id})
			if err == nil {
				rt.Fatalf("GetNote(%q) should return error after folder deletion", id)
			}
			if connect.CodeOf(err) != connect.CodeNotFound {
				rt.Fatalf("GetNote(%q): expected CodeNotFound, got %v", id, connect.CodeOf(err))
			}
		}

		// Verify: all task lists return NotFound via GetTaskList
		for _, id := range taskListIDs {
			_, err := taskSrv.GetTaskList(ctx, &taskspb.GetTaskListRequest{Id: id})
			if err == nil {
				rt.Fatalf("GetTaskList(%q) should return error after folder deletion", id)
			}
			if connect.CodeOf(err) != connect.CodeNotFound {
				rt.Fatalf("GetTaskList(%q): expected CodeNotFound, got %v", id, connect.CodeOf(err))
			}
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 12: Folder Rename Updates Parent Dir
// Validates: Requirements 7.1, 7.2, 7.3, 7.4, 14.4
func TestProperty_FolderRenameUpdatesParentDir(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		db := file.NewTestDB(t)
		logger := file.NopLogger()
		taskSrv := tasks.NewTaskServer(dataDir, db, logger)
		noteSrv := notes.NewNotesServer(dataDir, db, logger)
		fileSrv := file.NewFileServer(dataDir, db, logger)
		ctx := context.Background()

		// Generate old and new folder names (ensure different)
		oldName := folderNameGen().Draw(rt, "oldName")
		newName := folderNameGen().Draw(rt, "newName")
		for strings.EqualFold(oldName, newName) {
			newName = folderNameGen().Draw(rt, "newName-retry")
		}

		// Create the folder
		_, err := fileSrv.CreateFolder(ctx, &filev1.CreateFolderRequest{
			ParentDir: "",
			Name:      oldName,
		})
		if err != nil {
			rt.Fatalf("CreateFolder failed: %v", err)
		}

		// Create 1-2 notes in that folder
		numNotes := rapid.IntRange(1, 2).Draw(rt, "numNotes")
		for i := 0; i < numNotes; i++ {
			title := folderNameGen().Draw(rt, fmt.Sprintf("noteTitle-%d", i))
			content := noteContentGen().Draw(rt, fmt.Sprintf("noteContent-%d", i))
			_, err := noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
				ParentDir: oldName,
				Title:     title,
				Content:   content,
			})
			if err != nil {
				rt.Fatalf("CreateNote[%d] failed: %v", i, err)
			}
		}

		// Create 1 task list in that folder
		taskListTitle := folderNameGen().Draw(rt, "taskListTitle")
		_, err = taskSrv.CreateTaskList(ctx, &taskspb.CreateTaskListRequest{
			Title:     taskListTitle,
			ParentDir: oldName,
			Tasks: []*taskspb.MainTask{
				{Description: "Task", IsDone: false},
			},
			IsAutoDelete: false,
		})
		if err != nil {
			rt.Fatalf("CreateTaskList failed: %v", err)
		}

		// Rename the folder
		_, err = fileSrv.UpdateFolder(ctx, &filev1.UpdateFolderRequest{
			FolderPath: oldName,
			NewName:    newName,
		})
		if err != nil {
			rt.Fatalf("UpdateFolder failed: %v", err)
		}

		// Verify: ListNotes(newName) returns the notes
		notesResp, err := noteSrv.ListNotes(ctx, &notespb.ListNotesRequest{
			ParentDir: newName,
		})
		if err != nil {
			rt.Fatalf("ListNotes(%q) failed: %v", newName, err)
		}
		if len(notesResp.Notes) != numNotes {
			rt.Fatalf("ListNotes(%q): expected %d notes, got %d", newName, numNotes, len(notesResp.Notes))
		}

		// Verify: old folder dir doesn't exist on disk
		oldDirPath := filepath.Join(dataDir, oldName)
		if _, err := os.Stat(oldDirPath); !os.IsNotExist(err) {
			rt.Fatalf("expected old folder %q to not exist on disk, got err: %v", oldName, err)
		}

		// Verify: ListTaskLists(newName) returns the task list
		tlResp, err := taskSrv.ListTaskLists(ctx, &taskspb.ListTaskListsRequest{
			ParentDir: newName,
		})
		if err != nil {
			rt.Fatalf("ListTaskLists(%q) failed: %v", newName, err)
		}
		if len(tlResp.TaskLists) != 1 {
			rt.Fatalf("ListTaskLists(%q): expected 1 task list, got %d", newName, len(tlResp.TaskLists))
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 13: ListFiles Hybrid Discovery
// Validates: Requirements 3.4, 3.5, 11.1, 11.2, 11.3, 11.6, 11.7
func TestProperty_ListFilesHybridDiscovery(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		db := file.NewTestDB(t)
		logger := file.NopLogger()
		taskSrv := tasks.NewTaskServer(dataDir, db, logger)
		noteSrv := notes.NewNotesServer(dataDir, db, logger)
		fileSrv := file.NewFileServer(dataDir, db, logger)
		ctx := context.Background()

		// Generate a folder name and create it on disk
		folderName := folderNameGen().Draw(rt, "folderName")
		if err := os.MkdirAll(filepath.Join(dataDir, folderName), 0o755); err != nil {
			rt.Fatalf("failed to create folder on disk: %v", err)
		}

		// Create 1-2 notes in root ("") via noteSrv
		numNotes := rapid.IntRange(1, 2).Draw(rt, "numNotes")
		noteIDs := make([]string, 0, numNotes)
		for i := 0; i < numNotes; i++ {
			title := folderNameGen().Draw(rt, fmt.Sprintf("noteTitle-%d", i))
			content := noteContentGen().Draw(rt, fmt.Sprintf("noteContent-%d", i))
			resp, err := noteSrv.CreateNote(ctx, &notespb.CreateNoteRequest{
				ParentDir: "",
				Title:     title,
				Content:   content,
			})
			if err != nil {
				rt.Fatalf("CreateNote[%d] failed: %v", i, err)
			}
			noteIDs = append(noteIDs, resp.Note.Id)
		}

		// Create 1-2 task lists in root ("") via taskSrv
		numTaskLists := rapid.IntRange(1, 2).Draw(rt, "numTaskLists")
		for i := 0; i < numTaskLists; i++ {
			title := folderNameGen().Draw(rt, fmt.Sprintf("taskListTitle-%d", i))
			_, err := taskSrv.CreateTaskList(ctx, &taskspb.CreateTaskListRequest{
				Title:     title,
				ParentDir: "",
				Tasks: []*taskspb.MainTask{
					{Description: "Task A", IsDone: true},
					{Description: "Task B", IsDone: false},
				},
				IsAutoDelete: false,
			})
			if err != nil {
				rt.Fatalf("CreateTaskList[%d] failed: %v", i, err)
			}
		}

		// Call ListFiles("")
		resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Verify: folder appears with FOLDER type
		foundFolder := false
		for _, entry := range resp.Entries {
			if entry.ItemType == filev1.ItemType_ITEM_TYPE_FOLDER && entry.Title == folderName {
				foundFolder = true
				break
			}
		}
		if !foundFolder {
			rt.Fatalf("expected folder %q to appear in ListFiles with FOLDER type", folderName)
		}

		// Verify: notes appear with NOTE type and have preview in metadata
		noteCount := 0
		for _, entry := range resp.Entries {
			if entry.ItemType == filev1.ItemType_ITEM_TYPE_NOTE {
				noteCount++
				noteMeta := entry.GetNoteMetadata()
				if noteMeta == nil {
					rt.Fatalf("note entry %q has nil NoteMetadata", entry.Title)
				}
				if noteMeta.Preview == "" {
					rt.Fatalf("note entry %q has empty preview", entry.Title)
				}
				if noteMeta.Id == "" {
					rt.Fatalf("note entry %q has empty ID in metadata", entry.Title)
				}
				// Verify: note paths end with _<id>.md
				expectedSuffix := "_" + noteMeta.Id + ".md"
				if !strings.HasSuffix(entry.Path, expectedSuffix) {
					rt.Fatalf("note path %q does not end with %q", entry.Path, expectedSuffix)
				}
			}
		}
		if noteCount != numNotes {
			rt.Fatalf("expected %d note entries, got %d", numNotes, noteCount)
		}

		// Verify: task lists appear with TASK_LIST type and have counts in metadata
		taskListCount := 0
		for _, entry := range resp.Entries {
			if entry.ItemType == filev1.ItemType_ITEM_TYPE_TASK_LIST {
				taskListCount++
				tlMeta := entry.GetTaskListMetadata()
				if tlMeta == nil {
					rt.Fatalf("task list entry %q has nil TaskListMetadata", entry.Title)
				}
				if tlMeta.TotalTaskCount != 2 {
					rt.Fatalf("task list entry %q: expected TotalTaskCount=2, got %d", entry.Title, tlMeta.TotalTaskCount)
				}
				if tlMeta.DoneTaskCount != 1 {
					rt.Fatalf("task list entry %q: expected DoneTaskCount=1, got %d", entry.Title, tlMeta.DoneTaskCount)
				}
			}
		}
		if taskListCount != numTaskLists {
			rt.Fatalf("expected %d task list entries, got %d", numTaskLists, taskListCount)
		}
	})
}

// Feature: test-suite-sqlite-rewrite, Property 14: Orphan Disk Files Excluded from ListFiles
// Validates: Requirements 11.4, 11.5
func TestProperty_OrphanDiskFilesExcludedFromListFiles(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		dataDir := t.TempDir()
		db := file.NewTestDB(t)
		logger := file.NopLogger()
		fileSrv := file.NewFileServer(dataDir, db, logger)
		ctx := context.Background()

		// Generate random names for orphan files
		numOrphans := rapid.IntRange(1, 3).Draw(rt, "numOrphans")
		orphanNames := make([]string, 0, numOrphans*2)

		for i := 0; i < numOrphans; i++ {
			name := folderNameGen().Draw(rt, fmt.Sprintf("orphanName-%d", i))

			// Create note_<name>.md files on disk (old format, no DB row)
			noteOrphan := fmt.Sprintf("note_%s.md", name)
			orphanPath := filepath.Join(dataDir, noteOrphan)
			if err := os.WriteFile(orphanPath, []byte("orphan note content"), 0o644); err != nil {
				rt.Fatalf("failed to write orphan note file: %v", err)
			}
			orphanNames = append(orphanNames, noteOrphan)

			// Create tasks_<name>.md files on disk (old format, no DB row)
			taskOrphan := fmt.Sprintf("tasks_%s.md", name)
			taskOrphanPath := filepath.Join(dataDir, taskOrphan)
			if err := os.WriteFile(taskOrphanPath, []byte("orphan task content"), 0o644); err != nil {
				rt.Fatalf("failed to write orphan task file: %v", err)
			}
			orphanNames = append(orphanNames, taskOrphan)
		}

		// Call ListFiles("")
		resp, err := fileSrv.ListFiles(ctx, &filev1.ListFilesRequest{ParentDir: ""})
		if err != nil {
			rt.Fatalf("ListFiles failed: %v", err)
		}

		// Verify: none of the orphan files appear in entries
		for _, entry := range resp.Entries {
			for _, orphan := range orphanNames {
				if strings.Contains(entry.Path, orphan) || strings.Contains(entry.Title, orphan) {
					rt.Fatalf("orphan file %q should NOT appear in ListFiles entries, but found entry: path=%q title=%q", orphan, entry.Path, entry.Title)
				}
			}
		}
	})
}
