# Implementation Plan: Test Suite SQLite Rewrite

## Overview

This plan rewrites the test suite for the `tasks/`, `notes/`, `file/`, and `database/` packages to align with the SQLite storage migration. The approach is:

1. Clean up old test files and stale rapid failure data
2. Create shared test helper infrastructure (helpers_export_test.go files)
3. Build database-layer tests first (foundational)
4. Build package-specific tests (tasks, notes, file) on top
5. Validate the full suite compiles and passes

## Tasks

- [x] 1. Clean up old test files and stale testdata
  - [x] 1.1 Delete all existing test files in the tasks/ package
    - Delete: `tasks/autodelete_e2e_property_test.go`, `tasks/autodelete_property_test.go`, `tasks/create_task_list_test.go`, `tasks/interface_verification_test.go`, `tasks/list_task_lists_test.go`, `tasks/rrule_test.go`, `tasks/task_server_property_test.go`, `tasks/tasklist_message_property_test.go`, `tasks/test_helpers_test.go`, `tasks/update_task_list_helpers_test.go`, `tasks/update_task_list_test.go`, `tasks/uuid_property_test.go`, `tasks/uuid_test.go`, `tasks/validate_limits_test.go`
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6_

  - [x] 1.2 Delete all existing test files in the notes/ package
    - Delete: `notes/content_limit_test.go`, `notes/create_note_property_test.go`, `notes/create_note_test.go`, `notes/delete_note_property_test.go`, `notes/delete_note_test.go`, `notes/get_note_test.go`, `notes/list_notes_property_test.go`, `notes/list_notes_test.go`, `notes/not_found_property_test.go`, `notes/note_round_trip_property_test.go`, `notes/path_traversal_property_test.go`, `notes/test_helpers_test.go`, `notes/update_note_property_test.go`, `notes/update_note_test.go`, `notes/uuid_property_test.go`, `notes/uuid_test.go`
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6_

  - [x] 1.3 Delete all existing test files in the file/ package
    - Delete: `file/create_folder_test.go`, `file/error_conditions_test.go`, `file/file_api_property_test.go`, `file/list_files_enrichment_property_test.go`, `file/list_files_helpers_test.go`, `file/list_files_property_test.go`, `file/list_files_test.go`, `file/rename_delete_test.go`, `file/test_helpers_test.go`
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6_

  - [x] 1.4 Remove stale rapid failure files from testdata directories
    - Delete all `.fail` files under `tasks/testdata/rapid/`, `notes/testdata/rapid/`, and `file/testdata/rapid/`
    - These reference test functions that no longer exist after the rewrite
    - _Requirements: 9.1_

- [x] 2. Create test helper infrastructure
  - [x] 2.1 Create tasks/helpers_export_test.go
    - Declare `package tasks` (internal test package)
    - Export `TestDB(t *testing.T) *database.Database` — creates in-memory SQLite DB via `database.New(filepath.Join(t.TempDir(), "test.db"))`, registers `t.Cleanup` for `db.Close()`
    - Export `NopLogger() *slog.Logger` — returns `slog.New(slog.NewTextHandler(io.Discard, nil))`
    - _Requirements: 10.1, 10.2, 10.3, 15.3, 15.6_

  - [x] 2.2 Create notes/helpers_export_test.go
    - Declare `package notes` (internal test package)
    - Export `TestDB(t *testing.T) *database.Database` — same pattern as tasks
    - Export `NopLogger() *slog.Logger` — same pattern as tasks
    - _Requirements: 10.1, 10.2, 10.4, 15.3, 15.6_

  - [x] 2.3 Create file/helpers_export_test.go
    - Declare `package file` (internal test package)
    - Export `TestDB(t *testing.T) *database.Database` — same pattern as tasks
    - Export `NopLogger() *slog.Logger` — same pattern as tasks
    - _Requirements: 10.1, 10.2, 10.5, 15.3, 15.6_

- [x] 3. Checkpoint - Verify helper infrastructure compiles
  - Run `go build ./tasks/...`, `go build ./notes/...`, `go build ./file/...` to confirm no compile errors
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Create database package tests
  - [x] 4.1 Create database/schema_test.go
    - Declare `package database_test`
    - Test schema idempotency: call `database.New(path)` twice on the same path, verify both succeed without error
    - Test that tables exist by inserting and querying data after schema creation
    - _Requirements: 14.6, 15.2_

  - [x] 4.2 Create database/task_lists_test.go
    - Declare `package database_test`
    - Test `CreateTaskList` — insert a task list with main tasks and subtasks, verify returned row matches params
    - Test `GetTaskList` — create then get, verify all fields match
    - Test `GetTaskList` with non-existent ID returns `database.ErrNotFound`
    - Test `UpdateTaskList` — create, update title/tasks, verify updated fields
    - Test `UpdateTaskList` with non-existent ID returns `database.ErrNotFound`
    - Test `DeleteTaskList` — create, delete, verify returns true; delete again returns false
    - Test `ListTaskLists` — create multiple in same parent_dir, verify all returned
    - Test `ListTaskLists` — verify filtering by parent_dir (items in other dirs not returned)
    - Test `ListTaskListsWithCounts` — create task list with mix of done/open tasks, verify counts
    - _Requirements: 14.1, 14.5, 15.2_

  - [x] 4.3 Create database/notes_test.go
    - Declare `package database_test`
    - Test `InsertNote` — insert a note, verify no error
    - Test `GetNote` — insert then get, verify all fields match
    - Test `GetNote` with non-existent ID returns `database.ErrNotFound`
    - Test `UpdateNote` — insert, update title/preview/updatedAt, verify via GetNote
    - Test `UpdateNote` with non-existent ID returns `database.ErrNotFound`
    - Test `DeleteNote` — insert, delete returns true; delete again returns false
    - Test `ListNotes` — insert multiple in same parent_dir, verify all returned
    - Test `ListNotes` — verify filtering by parent_dir
    - _Requirements: 14.2, 15.2_

  - [x] 4.4 Create database/cascade_test.go
    - Declare `package database_test`
    - Test `DeleteTaskList` cascades to tasks: create task list with tasks, delete task list, verify tasks table is empty for that list
    - Test `DeleteByParentDir` — create notes and task lists in a directory, call DeleteByParentDir, verify GetNote/GetTaskList return ErrNotFound
    - Test `DeleteByParentDir` with nested paths — items in subdirectories are also deleted
    - Test `RenameParentDir` — create notes and task lists in old path, rename, verify ListNotes/ListTaskLists with new path returns them, old path returns empty
    - Test `RenameParentDir` with nested paths — items in subdirectories have prefix updated
    - _Requirements: 14.3, 14.4, 14.5, 15.2_

- [x] 5. Checkpoint - Verify database tests pass
  - Run `go test ./database/...` and confirm all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Create tasks package tests
  - [x] 6.1 Create tasks/crud_test.go
    - Declare `package tasks_test`
    - Import `echolist-backend/tasks` and construct server via `tasks.NewTaskServer(t.TempDir(), tasks.TestDB(t), tasks.NopLogger())`
    - Test CreateTaskList: create with title and tasks, verify response has ID, title, tasks with IDs
    - Test GetTaskList: create then get by ID, verify all fields match
    - Test GetTaskList with non-existent UUID returns NotFound
    - Test UpdateTaskList: create, update title and tasks, verify response reflects changes
    - Test UpdateTaskList preserves existing task IDs when provided
    - Test UpdateTaskList assigns new UUIDs for tasks with empty IDs
    - Test DeleteTaskList: create, delete, verify GetTaskList returns NotFound
    - Test ListTaskLists: create multiple in same dir, verify all returned
    - Test ListTaskLists filters by parent_dir
    - _Requirements: 1.1, 1.4, 4.1, 4.2, 4.3, 4.4, 8.1, 15.1, 15.5_

  - [x] 6.2 Create tasks/validation_test.go
    - Declare `package tasks_test`
    - Test empty title returns InvalidArgument
    - Test title with null bytes returns InvalidArgument
    - Test path traversal in parent_dir (`../`) returns InvalidArgument
    - Test invalid UUID in GetTaskList/UpdateTaskList/DeleteTaskList returns InvalidArgument
    - Test auto-delete behavior: create auto-delete list with done non-recurring tasks, verify they are filtered on get
    - Test recurring task advancement: mark recurring task done, verify due date advances and is_done resets
    - Test duplicate task list name in same parent_dir returns AlreadyExists
    - _Requirements: 1.1, 1.2, 1.3, 8.1, 8.3, 8.4, 8.5, 8.6, 15.1_

  - [x] 6.3 Create tasks/property_test.go
    - Declare `package tasks_test`
    - Import `pgregory.net/rapid`
    - **Property 1: Task Create-Then-Get Round Trip**
    - **Validates: Requirements 1.1, 8.1**
    - **Property 3: All Generated IDs Are Valid UUIDv4** (for task list, main tasks, subtasks)
    - **Validates: Requirements 4.1, 4.2**
    - **Property 4: Task ID Stability on Update** (existing IDs preserved, empty IDs get new UUIDs)
    - **Validates: Requirements 4.3, 4.4, 8.2**
    - **Property 5: Auto-Delete Filtering of Done Non-Recurring Tasks**
    - **Validates: Requirements 8.3**
    - **Property 6: Recurring Task Advancement**
    - **Validates: Requirements 8.4**
    - **Property 7: Path Traversal Prevention** (tasks RPCs)
    - **Validates: Requirements 8.5**
    - **Property 8: Invalid UUID Rejection** (tasks RPCs)
    - **Validates: Requirements 8.6**
    - **Property 15: Delete-Then-Get Returns NotFound** (tasks)
    - **Validates: Requirements 1.4**
    - **Property 16: Duplicate Task List Name Returns AlreadyExists**
    - **Validates: Requirements 8.1**
    - **Property 17: ListTaskLists Parent Dir Filtering**
    - **Validates: Requirements 8.1**

- [x] 7. Checkpoint - Verify tasks tests pass
  - Run `go test ./tasks/...` and confirm all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Create notes package tests
  - [x] 8.1 Create notes/crud_test.go
    - Declare `package notes_test`
    - Import `echolist-backend/notes` and construct server via `notes.NewNotesServer(t.TempDir(), notes.TestDB(t), notes.NopLogger())`
    - Test CreateNote: create with title and content, verify response has ID, title, content
    - Test CreateNote: verify file exists on disk at `database.NotePath(parentDir, title, id)`
    - Test GetNote: create then get by ID, verify title and content match
    - Test GetNote with non-existent UUID returns NotFound
    - Test UpdateNote: create, update title and content, verify response reflects changes
    - Test UpdateNote: verify old file removed, new file exists at updated path
    - Test DeleteNote: create, delete, verify GetNote returns NotFound
    - Test DeleteNote: verify file removed from disk
    - Test ListNotes: create multiple in same dir, verify all returned with correct previews
    - Test ListNotes filters by parent_dir
    - _Requirements: 2.1, 2.4, 5.1, 5.2, 5.3, 12.1, 12.2, 12.3, 15.1, 15.5_

  - [x] 8.2 Create notes/validation_test.go
    - Declare `package notes_test`
    - Test empty title returns InvalidArgument
    - Test title with null bytes returns InvalidArgument
    - Test path traversal in parent_dir returns InvalidArgument
    - Test invalid UUID in GetNote/UpdateNote/DeleteNote returns InvalidArgument
    - Test content exceeding limit returns InvalidArgument
    - Test preview computation: content > 100 runes → preview is first 100 runes
    - Test preview computation: content ≤ 100 runes → preview is full content
    - Test preview recomputation on update
    - _Requirements: 2.1, 2.2, 2.3, 5.1, 5.2, 5.3, 5.4, 8.5, 8.6, 15.1_

  - [x] 8.3 Create notes/property_test.go
    - Declare `package notes_test`
    - Import `pgregory.net/rapid`
    - **Property 2: Note Create-Then-Get Round Trip**
    - **Validates: Requirements 2.1, 8.1**
    - **Property 3: All Generated IDs Are Valid UUIDv4** (note IDs)
    - **Validates: Requirements 4.1, 4.2**
    - **Property 7: Path Traversal Prevention** (notes RPCs)
    - **Validates: Requirements 8.5**
    - **Property 8: Invalid UUID Rejection** (notes RPCs)
    - **Validates: Requirements 8.6**
    - **Property 9: Note Preview Computation**
    - **Validates: Requirements 5.1, 5.2, 5.3, 5.4**
    - **Property 10: Note File Path Convention**
    - **Validates: Requirements 2.4, 12.1, 12.2, 12.3**
    - **Property 15: Delete-Then-Get Returns NotFound** (notes)
    - **Validates: Requirements 1.4**
    - **Property 17: ListNotes Parent Dir Filtering**
    - **Validates: Requirements 8.1**

- [x] 9. Checkpoint - Verify notes tests pass
  - Run `go test ./notes/...` and confirm all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 10. Create file package tests
  - [x] 10.1 Create file/list_files_test.go
    - Declare `package file_test`
    - Import `echolist-backend/file`, `echolist-backend/tasks`, `echolist-backend/notes`
    - Construct all three servers sharing the same `db` and `dataDir`
    - Test ListFiles returns folder entries from filesystem (create dirs with `os.MkdirAll`)
    - Test ListFiles returns note entries from SQLite (create via NotesServer RPC)
    - Test ListFiles returns task list entries from SQLite (create via TaskServer RPC)
    - Test ListFiles note entries include preview field
    - Test ListFiles task list entries include total_task_count and done_task_count
    - Test ListFiles note entries use `<title>_<id>.md` path format
    - Test orphan `note_<title>.md` file on disk without DB row does NOT appear in ListFiles
    - Test orphan `tasks_<title>.md` file on disk without DB row does NOT appear in ListFiles
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7, 15.1_

  - [x] 10.2 Create file/folder_ops_test.go
    - Declare `package file_test`
    - Import `echolist-backend/file`, `echolist-backend/tasks`, `echolist-backend/notes`
    - Test CreateFolder: create folder, verify it appears in ListFiles
    - Test DeleteFolder: create folder with notes and task lists inside, delete folder, verify:
      - Folder removed from disk
      - Notes removed from SQLite (GetNote returns NotFound)
      - Task lists removed from SQLite (GetTaskList returns NotFound)
    - Test DeleteFolder with nested subfolders: all nested items removed
    - Test UpdateFolder (rename): create folder with notes and task lists, rename, verify:
      - ListNotes/ListTaskLists with new path returns items
      - ListNotes/ListTaskLists with old path returns empty
    - Test UpdateFolder with nested subfolders: nested items have parent_dir prefix updated
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 7.1, 7.2, 7.3, 7.4, 15.1_

  - [x] 10.3 Create file/property_test.go
    - Declare `package file_test`
    - Import `pgregory.net/rapid`
    - **Property 11: Folder Cascade Delete Removes All DB Rows**
    - **Validates: Requirements 6.1, 6.2, 6.3, 6.4, 14.3**
    - **Property 12: Folder Rename Updates Parent Dir**
    - **Validates: Requirements 7.1, 7.2, 7.3, 7.4, 14.4**
    - **Property 13: ListFiles Hybrid Discovery**
    - **Validates: Requirements 3.4, 3.5, 11.1, 11.2, 11.3, 11.6, 11.7**
    - **Property 14: Orphan Disk Files Excluded from ListFiles**
    - **Validates: Requirements 11.4, 11.5**

- [x] 11. Checkpoint - Verify file tests pass
  - Run `go test ./file/...` and confirm all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 12. Final validation
  - [x] 12.1 Run full test suite
    - Execute `go test ./...` and confirm all tests pass with zero failures
    - Verify no compilation errors across the entire project
    - _Requirements: 16.1, 16.2, 16.3, 16.4, 16.5, 16.6_

  - [x] 12.2 Verify common/ and auth/ packages are untouched
    - Confirm no test files in `common/` or `auth/` were modified
    - _Requirements: 13.1, 13.2_

- [x] 13. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks are mandatory
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties using `pgregory.net/rapid`
- The database package tests don't need helpers_export_test.go since they call `database.New()` directly
- The file/ package tests import both tasks and notes packages to create cross-package test data
- All tests use in-memory SQLite and `t.TempDir()` for isolation — no external services needed
