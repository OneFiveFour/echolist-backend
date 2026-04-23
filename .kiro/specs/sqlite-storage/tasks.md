# Implementation Plan: SQLite Storage Migration

## Overview

This plan migrates the echolist-backend from markdown-file + JSON-registry persistence to a single SQLite database for task lists, main tasks, subtasks, and note metadata. Note content remains on disk as markdown files. The plan is ordered so the codebase compiles and ideally passes tests at every checkpoint: foundation first, then proto/domain changes, then RPC rewrites, then integration, then old code removal, then wiring.

## Tasks

- [x] 1. Add SQLite dependency and create the `database` package
  - [x] 1.1 Add `modernc.org/sqlite` dependency to `go.mod`
    - Run `go get modernc.org/sqlite` to add the pure-Go SQLite driver
    - _Requirements: 1.5_

  - [x] 1.2 Create `database/database.go` with `Database` struct, `New`, `Close`, and `HealthCheck`
    - Create the `database` package at `database/database.go`
    - Implement `Database` struct wrapping `*sql.DB`
    - Implement `New(dbPath string) (*Database, error)` that opens/creates the SQLite database, enables WAL mode (`PRAGMA journal_mode=WAL`), enables foreign keys (`PRAGMA foreign_keys=ON`), and runs idempotent schema creation (`CREATE TABLE IF NOT EXISTS` for `task_lists`, `tasks`, and `notes` tables)
    - Implement `Close() error` to close the database connection
    - Implement `HealthCheck() error` that runs `SELECT 1`
    - Schema must match the design: `task_lists` (id TEXT PK, title, parent_dir TEXT NOT NULL DEFAULT '', is_auto_delete INTEGER, created_at INTEGER, updated_at INTEGER), `tasks` (id TEXT PK, task_list_id TEXT FK → task_lists.id ON DELETE CASCADE, parent_task_id TEXT FK → tasks.id ON DELETE CASCADE, position INTEGER, description TEXT, is_done INTEGER, due_date TEXT nullable, recurrence TEXT nullable), `notes` (id TEXT PK, title, parent_dir TEXT NOT NULL DEFAULT '', preview TEXT NOT NULL DEFAULT '', created_at INTEGER, updated_at INTEGER)
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 2.9, 18.1_

  - [x] 1.3 Create `database/types.go` with row types and parameter structs
    - Define `TaskListRow`, `TaskRow`, `NoteRow`, `CreateTaskListParams`, `CreateTaskParams`, `UpdateTaskListParams`, `InsertNoteParams` as specified in the design
    - `TaskRow.IsDone` (not `Done`), `TaskListRow.IsAutoDelete`
    - `TaskRow.TaskListId` and `TaskRow.ParentTaskId` are `*string` (nullable)
    - `TaskRow.DueDate` and `TaskRow.Recurrence` are `*string` (nullable)
    - `TaskListRow.ParentDir` is `string` with `""` for root (not `*string`)
    - _Requirements: 2.1, 2.3, 2.7_

  - [x] 1.4 Implement task list database operations
    - Implement `CreateTaskList(params CreateTaskListParams) (TaskListRow, []TaskRow, error)` — single transaction: insert task_lists row, insert main task rows (task_list_id set, parent_task_id NULL, position 0..N), insert subtask rows (parent_task_id set, task_list_id NULL, position 0..N)
    - Implement `GetTaskList(id string) (TaskListRow, []TaskRow, error)` — query task_lists + tasks, order by position
    - Implement `UpdateTaskList(params UpdateTaskListParams) (TaskListRow, []TaskRow, error)` — single transaction: update task_lists row, delete existing tasks (cascade handles subtasks), insert new tasks
    - Implement `DeleteTaskList(id string) (bool, error)` — delete task_lists row (cascade deletes tasks), return false if not found
    - Implement `ListTaskLists(parentDir string) ([]TaskListRow, map[string][]TaskRow, error)` — query by parent_dir
    - Implement `ListTaskListsWithCounts(parentDir string) ([]TaskListRow, error)` — aggregate query with total/done task counts for FileServer
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 8.1, 8.2, 8.3, 8.4, 9.1, 9.2, 10.1, 10.2, 10.3, 11.1, 11.2, 11.3, 13.2_

  - [x] 1.5 Implement note metadata database operations
    - Implement `InsertNote(params InsertNoteParams) error`
    - Implement `GetNote(id string) (NoteRow, error)` — return sql.ErrNoRows-based not-found
    - Implement `UpdateNote(id string, title string, preview string, updatedAt int64) error`
    - Implement `DeleteNote(id string) (bool, error)` — return false if not found
    - Implement `ListNotes(parentDir string) ([]NoteRow, error)` — query by parent_dir
    - _Requirements: 20.1, 21.1, 22.1, 23.1, 24.1_

  - [x] 1.6 Implement folder cascade and child count operations
    - Implement `CountChildrenInDir(parentDir string) (int, error)` — count notes + task_lists where parent_dir matches
    - Implement `DeleteByParentDir(dirPath string) error` — delete notes and task_lists where parent_dir = dirPath OR starts with dirPath + "/"
    - Implement `RenameParentDir(oldPath, newPath string) error` — update parent_dir prefix in notes and task_lists tables
    - _Requirements: 13.7, 29.1, 29.2, 30.1, 30.2_

- [x] 2. Checkpoint — Verify `database` package compiles
  - Run `go build ./database/...` to ensure the new package compiles
  - Ensure all tests pass, ask the user if questions arise.

- [x] 3. Protobuf schema changes and code generation
  - [x] 3.1 Update `proto/tasks/v1/tasks.proto`
    - Add `string id = 1` to `SubTask`, shift `description` to 2, rename `done` to `is_done` at field 3
    - Add `string id = 1` to `MainTask`, shift `description` to 2, rename `done` to `is_done` at field 3, shift `due_date` to 4, `recurrence` to 5, `sub_tasks` to 6
    - In `TaskList`, replace `string file_path = 2` with `string parent_dir = 2`
    - _Requirements: 3.1, 3.2, 3.3, 3.4_

  - [x] 3.2 Update `proto/notes/v1/notes.proto`
    - Remove `file_path` field from `Note` message
    - Renumber: `title` = 2, `content` = 3, `updated_at` = 4
    - _Requirements: 3.5_

  - [x] 3.3 Run `buf generate` to regenerate Go protobuf code
    - Run `buf generate` from the `proto/` directory to regenerate `proto/gen/` files
    - Verify generated code compiles
    - _Requirements: 3.1, 3.2, 3.3, 3.5_

- [x] 4. Domain type changes and UUID consolidation
  - [x] 4.1 Update `tasks/types.go` — add `Id` field, rename `Done` to `IsDone`
    - Add `Id string` as first field in `MainTask` struct
    - Add `Id string` as first field in `SubTask` struct
    - Rename `Done` to `IsDone` in both `MainTask` and `SubTask`
    - _Requirements: 4.1, 4.2_

  - [x] 4.2 Consolidate UUID validation into `common/uuid.go`
    - Create `common/uuid.go` with exported `ValidateUuidV4(id string) error` function (move from `tasks/uuid.go`)
    - Update `tasks/uuid.go` to call `common.ValidateUuidV4` (or remove and update callers)
    - Update `notes/uuid.go` to call `common.ValidateUuidV4` (or remove and update callers)
    - _Requirements: 31.1, 31.2_

  - [x] 4.3 Create `NotePath` helper function
    - Create a function (in `database` or `common` package) `NotePath(parentDir, title, id string) string` that returns `<title>_<id>.md` when parentDir is `""`, or `<parentDir>/<title>_<id>.md` otherwise
    - _Requirements: 16.1, 16.4, 2.9_

  - [x] 4.4 Fix all compilation errors from `Done` → `IsDone` rename and proto field changes
    - Update `tasks/validate.go`: `t.Done` → `t.IsDone`, `st.Done` → `st.IsDone`
    - Update `tasks/task_server.go`: `protoToMainTasks` and `protoToSubtasks` to map `Id` field and use `IsDone` (proto field is now `IsIsDone` or `GetIsIsDone` — check generated code); update `mainTasksToProto`, `subtasksToProto`, `buildTaskList` to use `ParentDir` instead of `FilePath`
    - Update `tasks/update_task_list.go`: `filterAutoDeleted` to use `IsDone`, `advanceRecurringTasks` to use `IsDone`
    - Update `tasks/create_task_list.go`: references to `Done` → `IsDone`
    - Update all note RPC files that reference `pb.Note{FilePath: ...}` — remove `FilePath` field
    - _Requirements: 4.1, 4.2, 4.3, 3.3, 3.5_

- [x] 5. Checkpoint — Verify compilation after proto and domain changes
  - Run `go build ./...` to ensure the entire project compiles with the new proto and domain types
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Rewrite task RPCs to use SQLite
  - [x] 6.1 Rewrite `TaskServer` struct and constructor
    - Replace `locks common.Locker` with `db *database.Database`
    - Update `NewTaskServer(dataDir string, db *database.Database, logger *slog.Logger) *TaskServer`
    - _Requirements: 12.1, 12.2, 12.3_

  - [x] 6.2 Rewrite `CreateTaskList` RPC
    - Remove all file I/O and registry code
    - Validate title, parent_dir, tasks (existing validation preserved)
    - Validate parent directory exists on disk via `common.RequireDir`
    - Generate TaskList_ID (UUIDv4), MainTask_IDs, SubTask_IDs
    - Compute next due date for recurring tasks
    - Call `db.CreateTaskList(...)` with all params in a single transaction
    - Build and return response with generated IDs, parent_dir (not file_path)
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 7.6, 5.1, 5.2, 5.3, 12.4, 17.1, 17.2, 17.3, 17.4, 17.5, 17.6, 17.7_

  - [x] 6.3 Rewrite `GetTaskList` RPC
    - Remove all file I/O, registry lookup, and parser calls
    - Validate ID, call `db.GetTaskList(id)`, reconstruct proto response from rows
    - Return MainTasks ordered by position, SubTasks ordered by position within each MainTask
    - _Requirements: 8.1, 8.2, 8.3, 8.4_

  - [x] 6.4 Rewrite `UpdateTaskList` RPC
    - Remove all file I/O, registry, rename, and persist logic
    - Validate ID, title, tasks; validate MainTask/SubTask IDs (non-empty must be valid UUIDv4)
    - Assign new IDs to tasks with empty `id` fields, preserve existing IDs
    - Advance recurring done tasks, apply auto-delete filtering
    - Call `db.UpdateTaskList(...)` in a single transaction
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 5.4, 5.5, 5.6, 5.7, 6.1, 6.2, 6.3, 17.1, 17.2, 17.3, 17.4, 17.5, 17.6, 17.7_

  - [x] 6.5 Rewrite `DeleteTaskList` RPC
    - Remove all file I/O and registry code
    - Validate ID, call `db.DeleteTaskList(id)`, return NotFound if not found
    - Cascade foreign keys handle task row deletion automatically
    - _Requirements: 10.1, 10.2, 10.3_

  - [x] 6.6 Rewrite `ListTaskLists` RPC
    - Remove all file scanning, registry reading, and parser calls
    - Validate parent_dir, call `db.ListTaskLists(parentDir)`, reconstruct proto response
    - _Requirements: 11.1, 11.2, 11.3_

- [x] 7. Checkpoint — Verify task RPCs compile
  - Run `go build ./tasks/...` to ensure the tasks package compiles
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 8. Rewrite note RPCs to use SQLite
  - [ ] 8.1 Rewrite `NotesServer` struct and constructor
    - Add `db *database.Database` field
    - Retain `locks common.Locker` for file I/O only
    - Update `NewNotesServer(dataDir string, db *database.Database, logger *slog.Logger) *NotesServer`
    - _Requirements: 25.1, 25.2, 25.3, 25.4, 25.5_

  - [ ] 8.2 Rewrite `CreateNote` RPC
    - Remove registry code
    - Generate Note_ID, compute file path via `NotePath(parentDir, title, id)`
    - Validate title, content, parent_dir; ensure parent dir exists on disk
    - Create file on disk first, then insert DB row via `db.InsertNote(...)`
    - If DB insert fails, delete file from disk (rollback)
    - Compute preview (first 100 runes of content)
    - Return response with Note_ID, title, content, updated_at (no file_path)
    - _Requirements: 20.1, 20.2, 20.3, 16.1, 16.4, 26.1, 17.8, 17.9, 17.10_

  - [ ] 8.3 Rewrite `GetNote` RPC
    - Remove registry lookup and `ValidateFileType` call
    - Query DB by Note_ID via `db.GetNote(id)`, compute file path via `NotePath`
    - Read content from disk; return NotFound if DB row missing or file missing
    - Return response with Note_ID, title, content, updated_at (no file_path)
    - _Requirements: 21.1, 21.2, 21.3, 21.4, 26.4_

  - [ ] 8.4 Rewrite `UpdateNote` RPC
    - Remove registry code
    - Query DB for current metadata, compute old file path via `NotePath`
    - If title changed: compute new file path, rename file on disk, update DB row; rollback rename if DB update fails
    - Write new content to file, update preview and updated_at in DB
    - Return NotFound if DB row missing or file missing on disk
    - Return response with Note_ID, title, content, updated_at (no file_path)
    - _Requirements: 22.1, 22.2, 22.3, 22.4, 22.5, 16.2, 26.3, 26.4_

  - [ ] 8.5 Rewrite `DeleteNote` RPC
    - Remove registry code
    - Query DB for metadata, compute file path via `NotePath`
    - Delete file from disk first, then delete DB row
    - If file missing but DB row exists, still delete DB row and return success
    - If DB delete fails after file removed, log error
    - _Requirements: 23.1, 23.2, 23.3, 23.4, 26.2_

  - [ ] 8.6 Rewrite `ListNotes` RPC
    - Remove directory scanning and registry code
    - Query DB via `db.ListNotes(parentDir)`, compute file paths via `NotePath`
    - Read content from disk for each note; skip notes whose files are missing
    - Return response with Note_ID, title, content, updated_at for each note
    - _Requirements: 24.1, 24.2, 24.3, 24.4, 26.5_

- [ ] 9. Checkpoint — Verify note RPCs compile
  - Run `go build ./notes/...` to ensure the notes package compiles
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 10. Rewrite `FileServer` and `ListFiles` for hybrid SQLite + filesystem
  - [ ] 10.1 Update `FileServer` struct and constructor
    - Add `db *database.Database` field
    - Update `NewFileServer(dataDir string, db *database.Database, logger *slog.Logger) *FileServer`
    - _Requirements: 13.4_

  - [ ] 10.2 Rewrite `ListFiles` RPC as hybrid filesystem + SQLite
    - Walk filesystem for folders only (via `os.ReadDir`, filter `e.IsDir()`)
    - Query `db.ListTaskListsWithCounts(parentDir)` for task list entries
    - Query `db.ListNotes(parentDir)` for note entries
    - Build folder entries: `child_count` = subdirectory count from `os.ReadDir` + `db.CountChildrenInDir(subdirRelPath)`
    - Build task list entries from DB results (id, title, updated_at, total_task_count, done_task_count)
    - Build note entries from DB results (id, title, updated_at, preview) — no file I/O needed
    - Remove `readRegistryReverse`, `noteRegistryPath`, `taskListRegistryPath` functions
    - Remove old `buildNoteEntry` and `buildTaskListEntry` that did file I/O
    - Rewrite `buildFolderEntry` to use `db.CountChildrenInDir` instead of `MatchesFileType`
    - _Requirements: 13.1, 13.2, 13.3, 13.5, 13.6, 13.7, 15.3, 15.4_

- [ ] 11. Rewrite `DeleteFolder` and `UpdateFolder` for SQLite cascade
  - [ ] 11.1 Rewrite `DeleteFolder` to cascade to SQLite
    - Before `os.RemoveAll`, call `db.DeleteByParentDir(folderRelPath)` to delete notes and task_lists rows
    - Use a transaction for DB deletes; if filesystem deletion fails, roll back DB transaction
    - _Requirements: 29.1, 29.2, 29.3, 29.4_

  - [ ] 11.2 Rewrite `UpdateFolder` to update SQLite parent directories
    - After `os.Rename` on disk, call `db.RenameParentDir(oldRelPath, newRelPath)` to update parent_dir prefix
    - If DB update fails, rename folder back on disk (rollback)
    - _Requirements: 30.1, 30.2, 30.3_

- [ ] 12. Checkpoint — Verify file package compiles
  - Run `go build ./file/...` to ensure the file package compiles
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 13. Remove old storage code
  - [ ] 13.1 Delete `tasks/parser.go`
    - _Requirements: 14.1_

  - [ ] 13.2 Delete `tasks/printer.go`
    - _Requirements: 14.2_

  - [ ] 13.3 Delete `tasks/registry.go`
    - _Requirements: 14.3_

  - [ ] 13.4 Delete `notes/registry.go`
    - _Requirements: 15.1_

  - [ ] 13.5 Delete `notes/title.go` (titles come from DB, not filename parsing)
    - _Requirements: 14.6_

  - [ ] 13.6 Delete `tasks/uuid.go` and `notes/uuid.go` (consolidated into `common/uuid.go`)
    - _Requirements: 31.2_

  - [ ] 13.7 Remove `common.TaskListFileType`, `common.NoteFileType`, `common.MatchesFileType`, `common.ExtractTitle`, `common.ValidateFileType` from `common/pathutil.go`
    - Remove the `FileType` struct, `NoteFileType`, `TaskListFileType` variables
    - Remove `ValidateFileType`, `MatchesFileType`, `ExtractTitle` functions
    - _Requirements: 14.4, 14.5_

  - [ ] 13.8 Remove `ExtractTaskListTitle` from `tasks/task_server.go`
    - _Requirements: 14.6_

  - [ ] 13.9 Clean up any remaining references to removed code
    - Search for any remaining imports or calls to deleted functions/types
    - Fix any compilation errors from removals
    - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.5, 15.1, 15.2_

- [ ] 14. Checkpoint — Verify full project compiles after removals
  - Run `go build ./...` to ensure the entire project compiles
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 15. Wire everything together in `main.go`
  - [ ] 15.1 Update `main.go` to initialize SQLite and pass DB to services
    - Open SQLite database via `database.New(filepath.Join(dataDir, "echolist.db"))`
    - If DB open fails, log error and `os.Exit(1)`
    - Pass `*database.Database` to `NewTaskServer(dataDir, db, logger)`
    - Pass `*database.Database` to `NewNotesServer(dataDir, db, logger)`
    - Pass `*database.Database` to `NewFileServer(dataDir, db, logger)`
    - Add `defer db.Close()` for clean shutdown
    - _Requirements: 19.1, 19.2, 19.3, 19.4, 19.5_

  - [ ] 15.2 Update `healthzHandler` to include database health check
    - Accept `*database.Database` parameter
    - Call `db.HealthCheck()` (runs `SELECT 1`)
    - On failure, add `"db: <error>"` to the unhealthy checks list and return HTTP 503
    - _Requirements: 18.1, 18.2, 18.3_

- [ ] 16. Final checkpoint — Full build and verification
  - Run `go build ./...` to ensure the entire project compiles
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Testing cleanup is explicitly deferred — no test tasks are included in this plan
- Each task references specific requirements for traceability
- Checkpoints ensure incremental compilation verification between major phases
- The `database` package centralizes all SQL; servers call typed methods
- Field naming follows the user's conventions: `Id` not `ID`, `IsDone` not `Done`
- `ParentDir` is `string` with `""` for root (not nullable `*string`)
- Unified `tasks` table (not separate main_tasks/sub_tasks)
- Note `file_path` removed from proto and all layers
- Note preview column in DB, computed on write
