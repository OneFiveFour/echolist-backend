# Requirements Document

## Introduction

The echolist-backend currently persists task lists as markdown files on disk (`tasks_<title>.md`) with a JSON registry file (`.tasklist_id_registry.json`) mapping UUIDs to file paths. MainTask and SubTask entities have no stable identifiers — they are identified only by position in the list.

Note content is stored as markdown files on disk (`note_<title>.md`) with a separate JSON registry file (`.note_id_registry.json`) mapping UUIDs to file paths. Note metadata (ID, file path) is tracked in this registry using the same read-all/modify/write-all pattern as the task registry.

This feature replaces the markdown-based task file persistence and JSON task registry with a single SQLite database. Tasks (MainTask, SubTask) become database rows with stable UUIDv4 identifiers. Task list metadata (ID, title, parent directory, isAutoDelete, updatedAt) moves from the JSON registry into SQLite. The markdown parser (`ParseTaskFile`) and printer (`PrintTaskFile`) are removed entirely.

Note content remains as markdown files on disk, but note metadata (ID, title, parent directory, timestamps) moves from the JSON registry into the same SQLite database. The note registry (`notes/registry.go`) and its JSON file (`.note_id_registry.json`) are removed. Note filenames change from `note_<title>.md` to `<title>_<id>.md`, incorporating the Note_ID to avoid filesystem collisions when multiple notes share the same title. The file path is not stored in the database — it is computed from `parent_dir`, `title`, and `id` using the filename convention.

Since this is pre-release software, no backwards compatibility or data migration from existing files is required.

## Glossary

- **TaskListService**: The gRPC/Connect service defined in `tasks.proto` that exposes CRUD operations for task lists.
- **TaskServer**: The Go struct in the `tasks` package that implements TaskListService, currently holding a `dataDir` string and a `common.Locker`.
- **TaskList**: A named collection of MainTasks, identified by a stable UUIDv4. Previously stored as a markdown file with a JSON registry entry; after this migration, stored as a row in the `task_lists` SQLite table.
- **MainTask**: A top-level task within a TaskList, represented by the `MainTask` protobuf message and the `MainTask` Go struct. After this migration, stored as a row in the `tasks` SQLite table with `task_list_id` set and `parent_task_id` NULL, and a stable UUIDv4 identifier.
- **SubTask**: A child task nested under a MainTask, represented by the `SubTask` protobuf message and the `SubTask` Go struct. After this migration, stored as a row in the `tasks` SQLite table with `parent_task_id` set and `task_list_id` NULL, and a stable UUIDv4 identifier.
- **MainTask_ID**: A stable UUIDv4 identifier assigned to a MainTask when first created. Remains constant for the lifetime of the MainTask regardless of reordering, editing, or sibling changes.
- **SubTask_ID**: A stable UUIDv4 identifier assigned to a SubTask when first created. Remains constant for the lifetime of the SubTask regardless of reordering, editing, or sibling changes.
- **TaskList_ID**: A stable UUIDv4 identifier assigned to a TaskList when first created via CreateTaskList_RPC.
- **App_DB**: The SQLite database file (`echolist.db`) stored in the data directory that holds all task and note metadata.
- **WAL_Mode**: SQLite Write-Ahead Logging mode, which allows concurrent readers and a single writer without external locking.
- **Parser**: The component (`ParseTaskFile`) that reads a markdown task file and produces domain objects. Removed by this migration.
- **Printer**: The component (`PrintTaskFile`) that serializes domain objects into markdown format. Removed by this migration.
- **Task_Registry**: The JSON file (`.tasklist_id_registry.json`) that maps TaskList_IDs to file paths. Removed by this migration.
- **Note_Registry**: The JSON file (`.note_id_registry.json`) that maps Note_IDs to file paths. Removed by this migration.
- **NoteService**: The gRPC/Connect service defined in `notes.proto` that exposes CRUD operations for notes (CreateNote, GetNote, UpdateNote, DeleteNote, ListNotes).
- **NotesServer**: The Go struct in the `notes` package that implements NoteService, currently holding a `dataDir` string, a `common.Locker`, and a `*slog.Logger`.
- **Note_ID**: A stable UUIDv4 identifier assigned to a Note when first created via CreateNote_RPC.
- **Note_Filename**: The filename format for note files on disk: `<title>_<Note_ID>.md`. The Note_ID suffix ensures uniqueness even when multiple notes share the same title in the same directory. The full file path is computed from `parent_dir`, `title`, and `Note_ID` — it is not stored in the database.
- **Note_Path_Helper**: A helper function that computes the absolute or relative file path for a note from its `parent_dir`, `title`, and `Note_ID` using the filename convention `<title>_<Note_ID>.md`.
- **CreateNote_RPC**: The RPC that creates a new note file on disk and inserts metadata into SQLite.
- **GetNote_RPC**: The RPC that retrieves a note by Note_ID, looking up metadata in SQLite and reading content from disk.
- **UpdateNote_RPC**: The RPC that updates a note's title and/or content, updating both the file on disk and the metadata in SQLite.
- **DeleteNote_RPC**: The RPC that deletes a note file from disk and removes its metadata from SQLite.
- **ListNotes_RPC**: The RPC that returns all notes under a given parent directory by querying SQLite for metadata and reading content from disk.
- **CreateTaskList_RPC**: The RPC that creates a new task list and returns the resulting TaskList message.
- **GetTaskList_RPC**: The RPC that retrieves a single task list by TaskList_ID.
- **UpdateTaskList_RPC**: The RPC that updates the tasks, title, or settings of an existing task list.
- **DeleteTaskList_RPC**: The RPC that deletes a task list by TaskList_ID.
- **ListTaskLists_RPC**: The RPC that returns all task lists under a given parent directory.
- **ListFiles_RPC**: The RPC in the `file` package that returns directory entries including task list and note metadata.
- **Pure_Go_SQLite_Driver**: The `modernc.org/sqlite` package, a pure Go SQLite implementation that requires no CGo and is compatible with `CGO_ENABLED=0` builds.

## Requirements

### Requirement 1: SQLite Database Initialization

**User Story:** As a backend developer, I want the application to create and configure a SQLite database on startup, so that task and note data has a reliable persistence layer.

#### Acceptance Criteria

1. WHEN the application starts, THE App_DB SHALL open or create a SQLite database file named `echolist.db` in the data directory.
2. WHEN the database is opened, THE App_DB SHALL enable WAL_Mode by executing `PRAGMA journal_mode=WAL`.
3. WHEN the database is opened, THE App_DB SHALL enable foreign key enforcement by executing `PRAGMA foreign_keys=ON`.
4. WHEN the database is opened, THE App_DB SHALL create the required tables (task and note tables) if they do not already exist (idempotent schema setup).
5. THE App_DB SHALL use the Pure_Go_SQLite_Driver (`modernc.org/sqlite`) so that the binary compiles with `CGO_ENABLED=0`.
6. IF the database file cannot be opened or the schema cannot be created, THEN THE application SHALL log the error and exit with a non-zero status code.

### Requirement 2: Database Schema

**User Story:** As a backend developer, I want a well-structured database schema for task lists, main tasks, subtasks, and note metadata, so that data integrity is enforced at the storage layer.

#### Acceptance Criteria

1. THE App_DB SHALL contain a `task_lists` table with columns: `id` (TEXT PRIMARY KEY, UUIDv4), `title` (TEXT NOT NULL), `parent_dir` (TEXT NOT NULL DEFAULT '', empty string represents the data-directory root), `is_auto_delete` (INTEGER NOT NULL DEFAULT 0), `created_at` (INTEGER NOT NULL, Unix milliseconds), `updated_at` (INTEGER NOT NULL, Unix milliseconds).
2. THE `task_lists` table SHALL NOT enforce a unique constraint on `(parent_dir, title)`. Multiple task lists with the same title in the same directory are permitted because each task list is identified by its stable TaskList_ID.
3. THE App_DB SHALL contain a `tasks` table with columns: `id` (TEXT PRIMARY KEY, UUIDv4), `task_list_id` (TEXT, nullable — foreign key to `task_lists.id`, set for top-level tasks, NULL for subtasks), `parent_task_id` (TEXT, nullable — foreign key to `tasks.id`, set for subtasks, NULL for top-level tasks), `position` (INTEGER NOT NULL), `description` (TEXT NOT NULL), `is_done` (INTEGER NOT NULL DEFAULT 0), `due_date` (TEXT, nullable — NULL when no due date is set), `recurrence` (TEXT, nullable — NULL when no recurrence rule is set).
4. A row in the `tasks` table is a MainTask when `task_list_id IS NOT NULL` and `parent_task_id IS NULL`. A row is a SubTask when `parent_task_id IS NOT NULL` and `task_list_id IS NULL`.
5. THE `tasks` table SHALL have a foreign key from `task_list_id` to `task_lists.id` with `ON DELETE CASCADE`.
6. THE `tasks` table SHALL have a self-referencing foreign key from `parent_task_id` to `tasks.id` with `ON DELETE CASCADE`.
7. THE App_DB SHALL contain a `notes` table with columns: `id` (TEXT PRIMARY KEY, UUIDv4), `title` (TEXT NOT NULL), `parent_dir` (TEXT NOT NULL DEFAULT '', empty string represents the data-directory root), `preview` (TEXT NOT NULL DEFAULT '', first 100 characters of note content, rune-safe), `created_at` (INTEGER NOT NULL, Unix milliseconds), `updated_at` (INTEGER NOT NULL, Unix milliseconds).
8. THE `notes` table SHALL NOT enforce a unique constraint on `(parent_dir, title)`. Multiple notes with the same title in the same directory are permitted because each note is identified by its stable Note_ID and has a unique filename incorporating the ID.
9. THE note file path SHALL NOT be stored in the database. It SHALL be computed at runtime from `parent_dir`, `title`, and `id` using the Note_Filename convention (`<title>_<id>.md`).

### Requirement 3: Protobuf Schema Changes

**User Story:** As a client developer, I want the MainTask and SubTask protobuf messages to include an `id` field, so that stable IDs are available in all API interactions.

#### Acceptance Criteria

1. THE MainTask protobuf message SHALL include a string field named `id` as field number 1, shifting existing fields accordingly.
2. THE SubTask protobuf message SHALL include a string field named `id` as field number 1, shifting existing fields accordingly.
3. THE TaskList protobuf message SHALL replace the `file_path` field (field number 2) with a string field named `parent_dir` that contains the relative directory path the task list belongs to.
4. THE `id` field SHALL be included in all API responses that contain MainTask or SubTask messages (CreateTaskList_RPC, GetTaskList_RPC, UpdateTaskList_RPC, ListTaskLists_RPC).
5. THE Note protobuf message SHALL remove the `file_path` field (field number 2). Notes are identified by their Note_ID, and the file path is an internal implementation detail computed via the Note_Path_Helper.

### Requirement 4: Domain Type Changes

**User Story:** As a backend developer, I want the Go domain types to carry the ID field, so that the ID flows through all internal processing without loss.

#### Acceptance Criteria

1. THE MainTask Go struct SHALL include a string field named `Id` as the first field in the struct, carrying the MainTask_ID.
2. THE SubTask Go struct SHALL include a string field named `Id` as the first field in the struct, carrying the SubTask_ID.
3. THE conversion functions between protobuf messages and domain types SHALL map the `id` field bidirectionally without loss.

### Requirement 5: MainTask and SubTask ID Generation

**User Story:** As a client developer, I want every MainTask and SubTask to have a unique stable ID, so that I can navigate to a task's detail screen and reliably apply results back to the correct task.

#### Acceptance Criteria

1. WHEN a TaskList is created via CreateTaskList_RPC, THE TaskListService SHALL assign a MainTask_ID to each MainTask in the request.
2. WHEN a TaskList is created via CreateTaskList_RPC, THE TaskListService SHALL assign a SubTask_ID to each SubTask in the request.
3. THE TaskListService SHALL generate each MainTask_ID and SubTask_ID as a version-4 UUID formatted as a lowercase hyphenated string (e.g. `550e8400-e29b-41d4-a716-446655440000`).
4. WHEN a TaskList is updated via UpdateTaskList_RPC and the client sends MainTasks with empty `id` fields, THE TaskListService SHALL assign new MainTask_IDs to those MainTasks.
5. WHEN a TaskList is updated via UpdateTaskList_RPC and the client sends SubTasks with empty `id` fields, THE TaskListService SHALL assign new SubTask_IDs to those SubTasks.
6. WHEN a TaskList is updated via UpdateTaskList_RPC and the client sends MainTasks with existing MainTask_IDs, THE TaskListService SHALL preserve those MainTask_IDs in the response and persisted data.
7. WHEN a TaskList is updated via UpdateTaskList_RPC and the client sends SubTasks with existing SubTask_IDs, THE TaskListService SHALL preserve those SubTask_IDs in the response and persisted data.

### Requirement 6: ID Validation

**User Story:** As a backend developer, I want incoming task IDs to be validated, so that malformed identifiers are rejected early with clear error messages.

#### Acceptance Criteria

1. WHEN an UpdateTaskListRequest contains a MainTask with a non-empty `id` field that is not a valid version-4 UUID, THE TaskListService SHALL return an InvalidArgument error.
2. WHEN an UpdateTaskListRequest contains a SubTask with a non-empty `id` field that is not a valid version-4 UUID, THE TaskListService SHALL return an InvalidArgument error.
3. THE TaskListService SHALL validate all task and subtask `id` fields before performing any database operations.

### Requirement 7: CreateTaskList RPC with SQLite

**User Story:** As a client developer, I want CreateTaskList to persist the new task list and all its tasks in SQLite, so that data is stored reliably without markdown files.

#### Acceptance Criteria

1. WHEN a valid CreateTaskListRequest is received, THE TaskListService SHALL insert a new row into the `task_lists` table with a generated TaskList_ID, the provided title, parent_dir, and is_auto_delete flag.
2. WHEN a valid CreateTaskListRequest is received, THE TaskListService SHALL insert rows into the `tasks` table for each MainTask, with generated MainTask_IDs, `task_list_id` set to the TaskList_ID, `parent_task_id` NULL, and sequential position values starting from 0.
3. WHEN a valid CreateTaskListRequest is received, THE TaskListService SHALL insert rows into the `tasks` table for each SubTask, with generated SubTask_IDs, `parent_task_id` set to the owning MainTask_ID, `task_list_id` NULL, and sequential position values starting from 0.
4. THE TaskListService SHALL insert the task list, main tasks, and sub tasks within a single database transaction.
5. WHEN a MainTask has a non-empty recurrence field, THE TaskListService SHALL compute the next due date and store it in the `due_date` column.
6. THE CreateTaskList response SHALL include the generated TaskList_ID, MainTask_IDs, and SubTask_IDs.

### Requirement 8: GetTaskList RPC with SQLite

**User Story:** As a client developer, I want GetTaskList to retrieve a complete task list from SQLite, so that I get all tasks with their stable IDs.

#### Acceptance Criteria

1. WHEN a valid GetTaskListRequest is received, THE TaskListService SHALL query the `task_lists` and `tasks` tables to reconstruct the full TaskList, distinguishing MainTasks (`task_list_id IS NOT NULL`) from SubTasks (`parent_task_id IS NOT NULL`).
2. THE TaskListService SHALL return MainTasks ordered by their `position` column.
3. THE TaskListService SHALL return SubTasks ordered by their `position` column within each MainTask.
4. IF the requested TaskList_ID does not exist in the database, THEN THE TaskListService SHALL return a NotFound error.

### Requirement 9: UpdateTaskList RPC with SQLite

**User Story:** As a client developer, I want UpdateTaskList to persist changes in SQLite while preserving stable IDs for existing tasks.

#### Acceptance Criteria

1. WHEN a valid UpdateTaskListRequest is received, THE TaskListService SHALL update the `task_lists` row with the new title, is_auto_delete flag, and updated_at timestamp.
2. THE TaskListService SHALL replace all tasks for the task list within a single transaction: delete existing rows from the `tasks` table (both main tasks and their subtasks via cascade), then insert the new set with preserved or newly generated IDs and updated position values.
3. WHEN is_auto_delete is true, THE TaskListService SHALL filter out done non-recurring MainTasks and done SubTasks before persisting.
4. THE TaskListService SHALL advance recurring tasks that are marked done by computing the next due date and resetting the done flag, using the existing task's due_date as the reference point.
5. IF the requested TaskList_ID does not exist in the database, THEN THE TaskListService SHALL return a NotFound error.

### Requirement 10: DeleteTaskList RPC with SQLite

**User Story:** As a client developer, I want DeleteTaskList to remove a task list and all its tasks from SQLite.

#### Acceptance Criteria

1. WHEN a valid DeleteTaskListRequest is received, THE TaskListService SHALL delete the row from the `task_lists` table.
2. THE cascade foreign keys SHALL automatically delete all associated task rows (both main tasks and subtasks) from the `tasks` table when a `task_lists` row is deleted.
3. IF the requested TaskList_ID does not exist in the database, THEN THE TaskListService SHALL return a NotFound error.

### Requirement 11: ListTaskLists RPC with SQLite

**User Story:** As a client developer, I want ListTaskLists to return all task lists in a directory from SQLite, so that directory browsing works without scanning markdown files.

#### Acceptance Criteria

1. WHEN a valid ListTaskListsRequest is received, THE TaskListService SHALL query the `task_lists` table filtered by the requested parent_dir.
2. THE TaskListService SHALL return each TaskList with all its MainTasks and SubTasks, ordered by position.
3. THE TaskListService SHALL include MainTask_IDs and SubTask_IDs in the response.

### Requirement 12: TaskServer Structural Changes

**User Story:** As a backend developer, I want TaskServer to use a database handle instead of file paths and locks, so that the implementation is clean and testable.

#### Acceptance Criteria

1. THE TaskServer struct SHALL replace the `locks` Locker field with a database handle (e.g. `*sql.DB` or a store interface) and SHALL retain the `dataDir` string field for parent directory validation.
2. THE `NewTaskServer` constructor SHALL accept a database handle in addition to the `dataDir` string.
3. THE TaskServer SHALL rely on SQLite WAL_Mode for concurrency instead of the per-path `common.Locker`.
4. THE TaskServer SHALL validate that the parent directory exists on disk (using `dataDir` and `common.RequireDir`) before creating a task list, so that task lists only appear in directories that `ListFiles` can discover.

### Requirement 13: File Package Integration

**User Story:** As a client developer, I want the ListFiles RPC to continue showing task list and note metadata, so that the file browser works correctly after the SQLite migration.

#### Acceptance Criteria

1. THE `ListFiles` RPC SHALL walk the filesystem directory for folders only, then query SQLite for notes and task lists where `parent_dir` matches the requested directory, and merge all result sets into the response.
2. THE `buildTaskListEntry` function SHALL be replaced: instead of reading and parsing a markdown file, it SHALL construct the FileEntry from a SQLite query result that provides the task list ID, title, updatedAt, totalTaskCount, and doneTaskCount.
3. THE `buildNoteEntry` function SHALL be replaced: instead of reading the filesystem and JSON registry, it SHALL construct the FileEntry entirely from a SQLite query result that provides the note ID, title, updatedAt, and preview. No file I/O is needed for note entries in ListFiles.
4. THE FileServer SHALL accept a database handle or a query interface in addition to `dataDir`, so it can retrieve task list and note metadata from SQLite.
5. THE `ListFiles` RPC SHALL no longer read the task list JSON registry (`.tasklist_id_registry.json`) or the note JSON registry (`.note_id_registry.json`).
6. THE `readRegistryReverse` function in `file/list_files.go` SHALL be removed since both JSON registries are replaced by SQLite.
7. THE `buildFolderEntry` function SHALL count task lists and notes in a subdirectory by querying SQLite (in addition to counting subfolders on disk) so that `child_count` remains accurate.

### Requirement 14: Removal of Old Task Storage Code

**User Story:** As a backend developer, I want the old markdown-based task storage code removed, so that there is a single source of truth for task data.

#### Acceptance Criteria

1. THE `tasks/parser.go` file (containing `ParseTaskFile`) SHALL be removed.
2. THE `tasks/printer.go` file (containing `PrintTaskFile`) SHALL be removed.
3. THE `tasks/registry.go` file (containing `registryRead`, `registryWrite`, `registryLookup`, `registryAdd`, `registryRemove`) SHALL be removed.
4. THE `common.TaskListFileType` constant SHALL be removed since task lists are no longer stored as files with the `tasks_` prefix and `.md` suffix.
5. THE `common.NoteFileType` constant SHALL be removed since note filenames now follow the `<title>_<Note_ID>.md` convention managed by the Note_Path_Helper rather than a prefix/suffix pattern.
6. THE `ExtractTaskListTitle` function in `tasks/task_server.go` SHALL be removed since titles are stored directly in the database.

### Requirement 15: Removal of Old Note Registry Code

**User Story:** As a backend developer, I want the old JSON-based note registry code removed, so that note metadata has a single source of truth in SQLite.

#### Acceptance Criteria

1. THE `notes/registry.go` file (containing `registryRead`, `registryWrite`, `registryLookup`, `registryAdd`, `registryRemove`, `registryPath`, and the `registryEntry` type) SHALL be removed.
2. THE `.note_id_registry.json` file SHALL no longer be read or written by any component.
3. THE `readRegistryReverse` function in `file/list_files.go` SHALL be removed since both JSON registries are gone.
4. THE `noteRegistryPath` and `taskListRegistryPath` helper functions in `file/list_files.go` SHALL be removed.

### Requirement 16: Note Filename Convention

**User Story:** As a user, I want note files on disk to be human-readable but collision-free, so that I can identify notes by browsing the filesystem even when multiple notes share the same title.

#### Acceptance Criteria

1. WHEN a note is created, THE NoteService SHALL construct the filename as `<title>_<Note_ID>.md` where `<title>` is the user-provided title and `<Note_ID>` is the full UUIDv4 string.
2. WHEN a note title is updated, THE NoteService SHALL rename the file from `<oldTitle>_<Note_ID>.md` to `<newTitle>_<Note_ID>.md`, preserving the same Note_ID suffix.
3. THE Note_ID suffix in the filename SHALL ensure filesystem-level uniqueness even when multiple notes have the same title in the same directory.
4. THE NoteService SHALL provide a Note_Path_Helper function that computes the relative file path from `parent_dir`, `title`, and `Note_ID`. All note file operations SHALL use this helper to derive paths rather than storing them in the database.

### Requirement 17: Validation Preservation

**User Story:** As a backend developer, I want all existing validation rules preserved, so that data integrity is maintained after the migration.

#### Acceptance Criteria

1. THE TaskListService SHALL enforce a maximum of 1000 MainTasks per TaskList.
2. THE TaskListService SHALL enforce a maximum of 100 SubTasks per MainTask.
3. THE TaskListService SHALL enforce a maximum of 1024 bytes for MainTask descriptions.
4. THE TaskListService SHALL enforce a maximum of 1024 bytes for SubTask descriptions.
5. THE TaskListService SHALL enforce that DueDate and Recurrence are mutually exclusive on a MainTask.
6. THE TaskListService SHALL validate RRULE strings using the `teambition/rrule-go` library.
7. THE TaskListService SHALL validate task list titles using `common.ValidateName`.
8. THE NoteService SHALL continue to enforce a maximum of 1 MiB for note content.
9. THE NoteService SHALL continue to validate note titles using `common.ValidateName`.
10. THE NoteService SHALL continue to validate parent directory paths using `common.ValidateParentDir`.

### Requirement 18: Health Check Updates

**User Story:** As an operator, I want the health check to verify database accessibility, so that monitoring detects storage problems.

#### Acceptance Criteria

1. THE healthz endpoint SHALL verify that the SQLite database is accessible by executing a lightweight query (e.g. `SELECT 1`).
2. THE healthz endpoint SHALL continue to verify data directory accessibility for note file storage.
3. IF the database health check fails, THEN THE healthz endpoint SHALL return HTTP 503 with a diagnostic message.

### Requirement 19: Application Startup Integration

**User Story:** As a backend developer, I want the database to be initialized in main.go and passed to the services that need it, so that the application wiring is clean.

#### Acceptance Criteria

1. WHEN the application starts, THE main function SHALL open the SQLite database and run schema initialization before creating service handlers.
2. THE main function SHALL pass the database handle to `NewTaskServer`.
3. THE main function SHALL pass the database handle (or a query interface) to `NewFileServer` so that `ListFiles` can query task list and note metadata.
4. THE main function SHALL pass the database handle to `NewNotesServer` so that note metadata is persisted in SQLite.
5. WHEN the application shuts down, THE main function SHALL close the database connection.

### Requirement 20: CreateNote RPC with SQLite

**User Story:** As a client developer, I want CreateNote to create the note file on disk and insert metadata into SQLite, so that note metadata is stored reliably without a JSON registry.

#### Acceptance Criteria

1. WHEN a valid CreateNoteRequest is received, THE NoteService SHALL generate a Note_ID, compute the file path using the Note_Path_Helper, create the note file on disk, and insert a metadata row into the `notes` table with the Note_ID, title, parent_dir, preview (first 100 characters of content, rune-safe), created_at, and updated_at.
2. THE NoteService SHALL create the file on disk first, then insert the database row. IF the database insert fails, THEN THE NoteService SHALL delete the file from disk and return an Internal error.
3. THE CreateNote response SHALL include the generated Note_ID, title, content, and updated_at.

### Requirement 21: GetNote RPC with SQLite

**User Story:** As a client developer, I want GetNote to look up note metadata in SQLite and read content from disk, so that retrieval is fast and consistent.

#### Acceptance Criteria

1. WHEN a valid GetNoteRequest is received, THE NoteService SHALL query the `notes` table by Note_ID to retrieve metadata (title, parent_dir, updated_at) and compute the file path using the Note_Path_Helper.
2. THE NoteService SHALL read the note content from the file at the computed file path on disk.
3. IF the requested Note_ID does not exist in the database, THEN THE NoteService SHALL return a NotFound error.
4. IF the database row exists but the file is missing from disk, THEN THE NoteService SHALL return a NotFound error.

### Requirement 22: UpdateNote RPC with SQLite

**User Story:** As a client developer, I want UpdateNote to update both the file on disk and the metadata in SQLite, so that title changes and content edits are persisted consistently.

#### Acceptance Criteria

1. WHEN a valid UpdateNoteRequest is received, THE NoteService SHALL query the `notes` table by Note_ID to retrieve the current metadata.
2. WHEN the title changes, THE NoteService SHALL compute the old and new file paths using the Note_Path_Helper, rename the file on disk, and update the `title` and `updated_at` columns in the `notes` table.
3. THE NoteService SHALL write the new content to the file on disk and update the `preview` (first 100 characters of new content, rune-safe) and `updated_at` columns in the `notes` table.
4. IF the database row exists but the file is missing from disk, THEN THE NoteService SHALL return a NotFound error.
5. IF the requested Note_ID does not exist in the database, THEN THE NoteService SHALL return a NotFound error.

### Requirement 23: DeleteNote RPC with SQLite

**User Story:** As a client developer, I want DeleteNote to remove the note file from disk and its metadata from SQLite.

#### Acceptance Criteria

1. WHEN a valid DeleteNoteRequest is received, THE NoteService SHALL query the `notes` table by Note_ID to retrieve metadata and compute the file path using the Note_Path_Helper.
2. THE NoteService SHALL delete the file from disk first, then delete the database row. IF the database delete fails after the file is removed, THE NoteService SHALL log the error (orphaned DB row is less harmful than an orphaned file).
3. IF the requested Note_ID does not exist in the database, THEN THE NoteService SHALL return a NotFound error.
4. IF the database row exists but the file is missing from disk, THEN THE NoteService SHALL still delete the database row and return success (cleanup of orphaned metadata).

### Requirement 24: ListNotes RPC with SQLite

**User Story:** As a client developer, I want ListNotes to query SQLite for note metadata and read content from disk, so that directory listing is efficient and consistent.

#### Acceptance Criteria

1. WHEN a valid ListNotesRequest is received, THE NoteService SHALL query the `notes` table filtered by the requested parent_dir to retrieve note metadata.
2. THE NoteService SHALL read the content of each note file from disk using the file path computed from the Note_Path_Helper.
3. IF a database row exists but the corresponding file is missing from disk, THEN THE NoteService SHALL skip that note in the response (do not fail the entire listing).
4. THE ListNotes response SHALL include Note_ID, title, content, and updated_at for each note.

### Requirement 25: NotesServer Structural Changes

**User Story:** As a backend developer, I want NotesServer to use a database handle for metadata operations while retaining dataDir for file I/O, so that the implementation is clean and testable.

#### Acceptance Criteria

1. THE NotesServer struct SHALL add a database handle field (e.g. `*sql.DB` or a store interface) for note metadata operations.
2. THE NotesServer struct SHALL retain the `dataDir` string field because note content files live on disk.
3. THE `NewNotesServer` constructor SHALL accept a database handle in addition to the `dataDir` string.
4. THE NotesServer SHALL replace the per-path `common.Locker` usage for registry operations with SQLite transactions for metadata operations.
5. THE NotesServer MAY retain file-level locking for actual file I/O operations (creating, writing, deleting markdown files on disk) where needed to prevent concurrent file access.

### Requirement 26: Note Disk-Database Consistency

**User Story:** As a backend developer, I want a defined ordering of disk and database operations for notes, so that failures leave the system in the least harmful inconsistent state.

#### Acceptance Criteria

1. WHEN creating a note, THE NoteService SHALL create the file on disk first, then insert the database row. IF the database insert fails, THEN THE NoteService SHALL delete the file from disk (rollback).
2. WHEN deleting a note, THE NoteService SHALL delete the file from disk first, then delete the database row. IF the database delete fails, THE NoteService SHALL log the error and return an Internal error (orphaned DB row is acceptable).
3. WHEN updating a note title (rename), THE NoteService SHALL compute the old and new file paths using the Note_Path_Helper, rename the file on disk first, then update the database row. IF the database update fails, THEN THE NoteService SHALL rename the file back to its original name (rollback).
4. IF a GetNote or UpdateNote request references a Note_ID that exists in the database but the file is missing from disk, THEN THE NoteService SHALL return a NotFound error.
5. IF a ListNotes query finds a database row whose file is missing from disk, THEN THE NoteService SHALL skip that note in the response rather than failing the entire listing.

### Requirement 27: Round-Trip Properties

**User Story:** As a developer, I want round-trip guarantees that creating entities and retrieving them produces identical data including stable IDs, so that the SQLite storage layer is trustworthy.

#### Acceptance Criteria

1. FOR ALL valid task list titles, parent directories, and task values, creating a TaskList via CreateTaskList_RPC and then retrieving the TaskList via GetTaskList_RPC SHALL produce MainTasks with the same MainTask_IDs, descriptions, done statuses, due dates, recurrences, and subtask counts that were returned in the CreateTaskList response.
2. FOR ALL valid task list titles, parent directories, and task values, creating a TaskList via CreateTaskList_RPC and then retrieving the TaskList via GetTaskList_RPC SHALL produce SubTasks with the same SubTask_IDs, descriptions, and done statuses that were returned in the CreateTaskList response.
3. FOR ALL valid task lists, updating a TaskList via UpdateTaskList_RPC and then retrieving it via GetTaskList_RPC SHALL produce the same tasks with the same IDs as returned in the UpdateTaskList response.
4. FOR ALL valid note titles, parent directories, and content values, creating a Note via CreateNote_RPC and then retrieving the Note via GetNote_RPC SHALL produce the same Note_ID, title, content, and updated_at that were returned in the CreateNote response.

### Requirement 28: Concurrency via WAL Mode

**User Story:** As a backend developer, I want SQLite WAL mode to handle concurrent access, so that the per-path locking mechanism can be removed for both task and note metadata operations.

#### Acceptance Criteria

1. THE App_DB SHALL operate in WAL_Mode to allow concurrent read access while a write is in progress.
2. THE TaskListService SHALL use database transactions for write operations (create, update, delete) to ensure atomicity.
3. THE TaskListService SHALL remove the per-path `common.Locker` usage for task list operations since SQLite provides its own concurrency control.
4. THE NoteService SHALL use database transactions for metadata write operations (insert, update, delete of note rows) to ensure atomicity.
5. THE NoteService SHALL remove the per-path `common.Locker` usage for registry operations since SQLite provides its own concurrency control for metadata.
6. THE NoteService MAY retain file-level locking for concurrent file I/O operations on note content files where needed.

### Requirement 29: Folder Delete Cascades to SQLite

**User Story:** As a client developer, I want deleting a folder to also remove all notes and task lists inside it from the database, so that the database stays consistent with the filesystem.

#### Acceptance Criteria

1. WHEN a folder is deleted via DeleteFolder_RPC, THE FileService SHALL delete all rows from the `notes` table where `parent_dir` equals the deleted folder path or starts with the deleted folder path followed by `/` (nested subdirectories).
2. WHEN a folder is deleted via DeleteFolder_RPC, THE FileService SHALL delete all rows from the `task_lists` table where `parent_dir` equals the deleted folder path or starts with the deleted folder path followed by `/` (nested subdirectories). Cascade foreign keys SHALL automatically delete associated task rows.
3. THE FileService SHALL delete the database rows within a single transaction, then delete the folder from disk via `os.RemoveAll`. IF the filesystem deletion fails, THE FileService SHALL roll back the database transaction.
4. THE FileService SHALL delete note files from disk as part of `os.RemoveAll` (they live inside the folder being deleted).

### Requirement 30: Folder Rename Updates SQLite Parent Directories

**User Story:** As a client developer, I want renaming a folder to update all notes and task lists inside it in the database, so that the database stays consistent with the filesystem.

#### Acceptance Criteria

1. WHEN a folder is renamed via UpdateFolder_RPC, THE FileService SHALL update the `parent_dir` column in the `notes` table for all rows where `parent_dir` equals the old folder path or starts with the old folder path followed by `/`, replacing the old path prefix with the new path prefix.
2. WHEN a folder is renamed via UpdateFolder_RPC, THE FileService SHALL update the `parent_dir` column in the `task_lists` table for all rows where `parent_dir` equals the old folder path or starts with the old folder path followed by `/`, replacing the old path prefix with the new path prefix.
3. THE FileService SHALL rename the folder on disk first, then update the database rows within a single transaction. IF the database update fails, THE FileService SHALL rename the folder back to its original name (rollback).

### Requirement 31: UUID Validation Consolidation

**User Story:** As a backend developer, I want a single shared UUID validation function, so that duplicate code is eliminated across packages.

#### Acceptance Criteria

1. THE `validateUuidV4` function SHALL be consolidated into a single shared location (e.g. `common/` package) and used by both the TaskListService and NoteService.
2. THE duplicate `validateUuidV4` implementations in `tasks/uuid.go` and `notes/uuid.go` SHALL be removed.
