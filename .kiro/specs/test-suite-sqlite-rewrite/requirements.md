# Requirements Document

## Introduction

The echolist-backend recently completed a major refactoring (sqlite-storage spec) that replaced markdown-file + JSON-registry persistence with SQLite. The source code compiles and the architecture is correct, but the test suite is broken because tests were written for the old storage model.

The old architecture stored task lists as `tasks_<title>.md` files and notes as `note_<title>.md` files on disk, with JSON registry files mapping UUIDs to file paths. Tests created data by writing files directly to disk and manipulating registry JSON — then called RPCs to verify behavior.

The new architecture stores task list metadata and tasks in SQLite, and note metadata (including preview) in SQLite. Note content lives on disk as `<title>_<id>.md` files. There are no more registry JSON files. The RPCs now read from/write to SQLite, so tests that create files on disk without inserting DB rows get empty results or NotFound errors.

This feature rewrites all failing tests in the `tasks/`, `notes/`, and `file/` packages to use the new SQLite-backed data creation paths, removes obsolete assertions about old file formats, and adds new assertions for SQLite-specific behaviors (ID generation, preview computation, cascade deletes, parent_dir updates).

## Glossary

- **Test_Suite**: The collection of Go test files across the `tasks/`, `notes/`, and `file/` packages that exercise the echolist-backend RPC handlers.
- **TaskServer**: The Go struct in the `tasks` package that implements the TaskListService gRPC/Connect handler, backed by `*database.Database`.
- **NotesServer**: The Go struct in the `notes` package that implements the NoteService gRPC/Connect handler, backed by `*database.Database` and disk files.
- **FileServer**: The Go struct in the `file` package that implements the FileService gRPC/Connect handler, discovering folders from disk and notes/task lists from SQLite.
- **testDB**: A test helper function present in each package that creates an in-memory SQLite database with full schema for use in tests.
- **RPC_Data_Creation**: The pattern of creating test data by calling RPC methods (CreateTaskList, CreateNote) rather than writing files to disk.
- **DB_Direct_Creation**: The pattern of creating test data by calling `database.Database` methods directly (InsertNote, CreateTaskList) on the test database.
- **Property_Test**: A test using `pgregory.net/rapid` that generates random valid inputs and verifies invariant properties hold across many iterations.
- **Integration_Test**: A test that creates a server with a real (temp) database, calls RPCs, and verifies responses.
- **Note_Filename**: The new filename format for note files on disk: `<title>_<id>.md`.
- **Old_Note_Filename**: The removed filename format: `note_<title>.md`.
- **Old_TaskList_Filename**: The removed filename format: `tasks_<title>.md`.
- **Preview**: The first 100 runes of note content, stored in the `notes` SQLite table.
- **Cascade_Delete**: SQLite foreign key behavior where deleting a parent row (task_list or folder) automatically removes child rows (tasks, notes, task lists in that folder).
- **Parent_Dir_Update**: The operation of updating the `parent_dir` column in SQLite when a folder is renamed.
- **External_Test_Package**: A Go test file that declares `package foo_test` instead of `package foo`, forcing tests to use only the exported API of the package under test. This provides black-box testing and cleaner separation between test and production code.
- **Internal_Test_Package**: A Go test file that declares the same package name as the production code (e.g., `package tasks`), giving access to unexported symbols. Used only when testing unexported internals is necessary.

## Requirements

### Requirement 1: Task Package Test Data Creation

**User Story:** As a developer, I want task tests to create data through the TaskServer RPC or database methods, so that tests exercise the actual SQLite-backed code paths.

#### Acceptance Criteria

1. WHEN a task test needs to create a task list, THE Test_Suite SHALL create data by calling `TaskServer.CreateTaskList` RPC or `database.CreateTaskList` directly on the testDB.
2. THE Test_Suite SHALL NOT create `tasks_<title>.md` files on disk to set up task test data.
3. THE Test_Suite SHALL NOT read or write `.tasklist_id_registry.json` files in any test.
4. WHEN a task test verifies deletion, THE Test_Suite SHALL verify that `GetTaskList` returns NotFound rather than checking for file absence on disk.
5. THE Test_Suite SHALL remove all assertions that check for `tasks_<title>.md` file existence on disk (e.g., `os.Stat(filepath.Join(tmp, "tasks_"+name+".md"))`).

### Requirement 2: Note Package Test Data Creation

**User Story:** As a developer, I want note tests to create data through the NotesServer RPC or database methods, so that tests exercise the actual SQLite-backed code paths.

#### Acceptance Criteria

1. WHEN a note test needs to create a note, THE Test_Suite SHALL create data by calling `NotesServer.CreateNote` RPC or by calling `database.InsertNote` and writing the file to disk at the correct path.
2. THE Test_Suite SHALL NOT create `note_<title>.md` files on disk to set up note test data.
3. THE Test_Suite SHALL NOT read or write `.note_id_registry.json` files in any test.
4. WHEN a note test verifies file creation, THE Test_Suite SHALL check for the new filename format `<title>_<id>.md` using `database.NotePath(parentDir, title, id)`.
5. THE Test_Suite SHALL remove all assertions on `note.FilePath` since the `file_path` field no longer exists on the Note proto message.

### Requirement 3: File Package Test Data Creation

**User Story:** As a developer, I want file/ListFiles tests to set up data via task and note RPCs, so that ListFiles correctly discovers entries from SQLite.

#### Acceptance Criteria

1. WHEN a ListFiles test needs notes to appear in the listing, THE Test_Suite SHALL create notes by calling `NotesServer.CreateNote` RPC or by inserting rows into the `notes` table via `database.InsertNote` and writing the corresponding file.
2. WHEN a ListFiles test needs task lists to appear in the listing, THE Test_Suite SHALL create task lists by calling `TaskServer.CreateTaskList` RPC or by inserting rows into the `task_lists` table via `database.CreateTaskList`.
3. THE Test_Suite SHALL NOT create `note_<title>.md` or `tasks_<title>.md` files on disk and expect them to appear in ListFiles results.
4. WHEN a ListFiles test verifies note entries, THE Test_Suite SHALL verify that the entry path matches the Note_Filename format (`<title>_<id>.md`) rather than the Old_Note_Filename format.
5. WHEN a ListFiles test verifies folder entries, THE Test_Suite SHALL continue to create folders on disk since folders are still discovered from the filesystem.

### Requirement 4: Task ID Generation Verification

**User Story:** As a developer, I want tests to verify that MainTask and SubTask IDs are generated on create and preserved on update, so that the stable ID contract is validated.

#### Acceptance Criteria

1. WHEN a task list is created via CreateTaskList RPC, THE Test_Suite SHALL verify that every MainTask in the response has a non-empty `Id` field containing a valid UUIDv4 string.
2. WHEN a task list is created via CreateTaskList RPC, THE Test_Suite SHALL verify that every SubTask in the response has a non-empty `Id` field containing a valid UUIDv4 string.
3. WHEN a task list is updated via UpdateTaskList RPC with MainTasks carrying existing IDs, THE Test_Suite SHALL verify that those IDs are preserved unchanged in the response.
4. WHEN a task list is updated via UpdateTaskList RPC with MainTasks carrying empty IDs, THE Test_Suite SHALL verify that new valid UUIDv4 IDs are assigned in the response.

### Requirement 5: Note Preview Verification

**User Story:** As a developer, I want tests to verify that the preview field is correctly computed and stored, so that ListFiles and ListNotes return accurate previews.

#### Acceptance Criteria

1. WHEN a note is created with content longer than 100 runes, THE Test_Suite SHALL verify that the preview returned by ListNotes contains exactly the first 100 runes of the content.
2. WHEN a note is created with content shorter than 100 runes, THE Test_Suite SHALL verify that the preview returned by ListNotes contains the full content.
3. WHEN a note is updated with new content, THE Test_Suite SHALL verify that the preview is recomputed from the new content.
4. WHEN ListFiles returns a note entry, THE Test_Suite SHALL verify that the preview field is populated and matches the expected first 100 runes of the note content.

### Requirement 6: Folder Cascade Delete Verification

**User Story:** As a developer, I want tests to verify that deleting a folder cascades to SQLite, so that orphaned database rows are not left behind.

#### Acceptance Criteria

1. WHEN a folder containing notes is deleted via DeleteFolder RPC, THE Test_Suite SHALL verify that the notes in that folder are removed from the SQLite `notes` table.
2. WHEN a folder containing task lists is deleted via DeleteFolder RPC, THE Test_Suite SHALL verify that the task lists in that folder are removed from the SQLite `task_lists` table.
3. WHEN a folder containing nested subfolders with notes and task lists is deleted, THE Test_Suite SHALL verify that all nested notes and task lists are removed from SQLite.
4. THE Test_Suite SHALL verify cascade deletion by attempting to retrieve the deleted items via GetNote or GetTaskList RPCs and confirming NotFound errors.

### Requirement 7: Folder Rename Parent Dir Update Verification

**User Story:** As a developer, I want tests to verify that renaming a folder updates `parent_dir` in SQLite, so that notes and task lists remain discoverable after a rename.

#### Acceptance Criteria

1. WHEN a folder is renamed via UpdateFolder RPC, THE Test_Suite SHALL verify that notes previously in that folder have their `parent_dir` updated to the new folder name.
2. WHEN a folder is renamed via UpdateFolder RPC, THE Test_Suite SHALL verify that task lists previously in that folder have their `parent_dir` updated to the new folder name.
3. WHEN a folder is renamed, THE Test_Suite SHALL verify that ListNotes and ListTaskLists with the new parent_dir return the items that were in the old folder.
4. WHEN a folder is renamed, THE Test_Suite SHALL verify that ListNotes and ListTaskLists with the old parent_dir return empty results.

### Requirement 8: Property Test Preservation and Migration

**User Story:** As a developer, I want existing property-based tests to be preserved where the property still holds, so that valuable invariant coverage is not lost.

#### Acceptance Criteria

1. THE Test_Suite SHALL preserve property tests for create-then-get round-trips, updating them to use RPC_Data_Creation instead of file manipulation.
2. THE Test_Suite SHALL preserve property tests for ID stability (IDs preserved across updates), updating the data creation path.
3. THE Test_Suite SHALL preserve property tests for auto-delete filtering of done non-recurring tasks.
4. THE Test_Suite SHALL preserve property tests for recurrence advancement (done recurring tasks get reset with next due date).
5. THE Test_Suite SHALL preserve property tests for path traversal prevention.
6. THE Test_Suite SHALL preserve property tests for invalid UUID rejection.
7. THE Test_Suite SHALL remove property tests that verify Old_TaskList_Filename or Old_Note_Filename patterns on disk (e.g., `TestProperty5_CreatedTaskListsUseTasksPrefix`).
8. THE Test_Suite SHALL remove property tests that verify JSON registry behavior (e.g., `TestProperty_ReadRegistryReverseInverse`).

### Requirement 9: Removal of Obsolete Test Assertions

**User Story:** As a developer, I want all obsolete assertions removed from tests, so that the test suite compiles and passes cleanly.

#### Acceptance Criteria

1. THE Test_Suite SHALL remove all assertions that check for `tasks_<title>.md` file existence or content on disk.
2. THE Test_Suite SHALL remove all assertions that check for `note_<title>.md` file existence (replaced by `<title>_<id>.md` checks where file verification is needed).
3. THE Test_Suite SHALL remove all assertions on `note.FilePath` or `taskList.FilePath` fields.
4. THE Test_Suite SHALL remove all references to `readRegistryReverse`, `noteRegistryPath`, `taskListRegistryPath`, or any registry-related helper functions.
5. THE Test_Suite SHALL remove all references to `common.TaskListFileType`, `common.NoteFileType`, `common.MatchesFileType`, or `common.ExtractTitle` in test files.
6. THE Test_Suite SHALL remove all test code that creates `.tasklist_id_registry.json` or `.note_id_registry.json` files.

### Requirement 10: Test Helper Preservation

**User Story:** As a developer, I want existing test helpers that are still valid to be preserved, so that test infrastructure is not unnecessarily rebuilt.

#### Acceptance Criteria

1. THE Test_Suite SHALL preserve the `testDB(t)` helper functionality that creates an in-memory SQLite database, adapting it to be accessible from External_Test_Package files.
2. THE Test_Suite SHALL preserve the `nopLogger()` helper functionality, adapting it to be accessible from External_Test_Package files.
3. THE Test_Suite SHALL preserve the `NewTaskServer(dataDir, testDB(t), nopLogger())` construction pattern, using the exported constructor from the external test package.
4. THE Test_Suite SHALL preserve the `NewNotesServer(dataDir, testDB(t), nopLogger())` construction pattern, using the exported constructor from the external test package.
5. THE Test_Suite SHALL preserve the `NewFileServer(dataDir, testDB(t), nopLogger())` construction pattern, using the exported constructor from the external test package.

### Requirement 11: ListFiles Hybrid Discovery Verification

**User Story:** As a developer, I want ListFiles tests to verify the hybrid discovery model (folders from disk, notes and task lists from SQLite), so that the merged result is correct.

#### Acceptance Criteria

1. THE Test_Suite SHALL verify that ListFiles returns folder entries discovered from the filesystem.
2. THE Test_Suite SHALL verify that ListFiles returns note entries discovered from the SQLite `notes` table (not from `note_*.md` files on disk).
3. THE Test_Suite SHALL verify that ListFiles returns task list entries discovered from the SQLite `task_lists` table (not from `tasks_*.md` files on disk).
4. THE Test_Suite SHALL verify that creating a `note_<title>.md` file on disk without a corresponding database row does NOT cause it to appear in ListFiles results.
5. THE Test_Suite SHALL verify that creating a `tasks_<title>.md` file on disk without a corresponding database row does NOT cause it to appear in ListFiles results.
6. WHEN ListFiles returns note entries, THE Test_Suite SHALL verify that the entry includes the note's preview field from SQLite.
7. WHEN ListFiles returns task list entries, THE Test_Suite SHALL verify that the entry includes total_task_count and done_task_count from SQLite.

### Requirement 12: Note File Path Verification

**User Story:** As a developer, I want note tests to verify the new filename convention, so that file operations use the correct path format.

#### Acceptance Criteria

1. WHEN a note is created, THE Test_Suite SHALL verify that the file on disk exists at the path computed by `database.NotePath(parentDir, title, id)`.
2. WHEN a note title is updated, THE Test_Suite SHALL verify that the old file (`<oldTitle>_<id>.md`) no longer exists and the new file (`<newTitle>_<id>.md`) exists with the correct content.
3. WHEN a note is deleted, THE Test_Suite SHALL verify that the file at `database.NotePath(parentDir, title, id)` no longer exists on disk.
4. THE Test_Suite SHALL NOT assert file paths using the `note_<title>.md` format in any test.

### Requirement 13: Common Package Test Preservation

**User Story:** As a developer, I want the common package tests to remain unchanged, so that stable utility code is not disrupted.

#### Acceptance Criteria

1. THE Test_Suite SHALL NOT modify any test files in the `common/` package.
2. THE Test_Suite SHALL NOT modify any test files in the `auth/` package.

### Requirement 14: Database Package Unit Tests

**User Story:** As a developer, I want unit tests for the database package, so that each store method is validated in isolation.

#### Acceptance Criteria

1. THE Test_Suite SHALL include unit tests for `database.CreateTaskList` verifying CRUD operations with an in-memory SQLite database.
2. THE Test_Suite SHALL include unit tests for `database.InsertNote`, `database.GetNote`, `database.UpdateNote`, `database.DeleteNote`, and `database.ListNotes` verifying CRUD operations.
3. THE Test_Suite SHALL include unit tests for `database.DeleteByParentDir` verifying that notes and task lists in a deleted directory are removed.
4. THE Test_Suite SHALL include unit tests for `database.RenameParentDir` verifying that `parent_dir` is updated for notes and task lists when a folder is renamed.
5. THE Test_Suite SHALL include unit tests for cascade delete behavior: deleting a task list removes all its tasks, deleting a main task removes its subtasks.
6. THE Test_Suite SHALL include unit tests for schema initialization idempotency (calling `database.New` twice on the same path succeeds).

### Requirement 15: External Test Packages and File Consolidation

**User Story:** As a developer, I want test files to use external test packages and be consolidated into fewer well-named files, so that the directory structure is easier to read and tests exercise only the public API.

#### Acceptance Criteria

1. ALL integration tests and property tests in the `tasks/`, `notes/`, and `file/` packages SHALL use External_Test_Package declarations (`package tasks_test`, `package notes_test`, `package file_test`).
2. ALL database package tests SHALL use an External_Test_Package declaration (`package database_test`).
3. Internal_Test_Package declarations (`package tasks`, `package notes`, `package file`) SHALL only be used for test helper files that need access to unexported symbols (e.g., `export_test.go` files that expose internals for external tests).
4. Test files in each package SHALL be consolidated into a small number of well-named files organized by concern:
   - `tasks/`: `crud_test.go` (create/get/update/delete integration tests), `property_test.go` (property-based tests), `validation_test.go` (input validation tests), `helpers_export_test.go` (exported test helpers if needed)
   - `notes/`: `crud_test.go`, `property_test.go`, `validation_test.go`, `helpers_export_test.go`
   - `file/`: `list_files_test.go` (ListFiles integration tests), `folder_ops_test.go` (create/delete/rename folder tests), `property_test.go`, `helpers_export_test.go`
   - `database/`: `task_lists_test.go`, `notes_test.go`, `schema_test.go`, `cascade_test.go`
5. WHEN an external test package needs to construct a server instance, THE Test_Suite SHALL import the package under test and call its exported constructor (e.g., `tasks.NewTaskServer(dataDir, db, logger)`).
6. THE `testDB(t)` and `nopLogger()` helpers SHALL be defined in a shared test helper file within each package's test files, accessible to the external test package.

### Requirement 16: Test Compilation and Pass Rate

**User Story:** As a developer, I want all rewritten tests to compile and pass, so that the CI pipeline is green.

#### Acceptance Criteria

1. WHEN `go test ./tasks/...` is executed, THE Test_Suite SHALL compile without errors and all tests SHALL pass.
2. WHEN `go test ./notes/...` is executed, THE Test_Suite SHALL compile without errors and all tests SHALL pass.
3. WHEN `go test ./file/...` is executed, THE Test_Suite SHALL compile without errors and all tests SHALL pass.
4. WHEN `go test ./database/...` is executed, THE Test_Suite SHALL compile without errors and all tests SHALL pass.
5. WHEN `go test ./...` is executed, THE Test_Suite SHALL compile without errors and all tests across the entire project SHALL pass.
6. ALL test files using External_Test_Package declarations SHALL compile correctly with proper imports of the package under test.
