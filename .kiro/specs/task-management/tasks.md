# Implementation Plan: Task Management

## Overview

Add task management to echolist-backend. The implementation proceeds bottom-up: shared utilities first, then migration of existing services, then the new task service built on top. All code is Go, using Connect-Go for RPC and `pgregory.net/rapid` for property-based testing.

## Tasks

- [x] 1. Extract shared `pathutil` package
  - [x] 1.1 Create `pathutil/pathutil.go` with `IsSubPath` and `ValidatePath` functions
    - Move `isSubPath` from `folder/folder_server.go` into `pathutil.IsSubPath`
    - Add `ValidatePath(dataDir, relativePath) (string, error)` that cleans and validates a path against the data directory root
    - _Requirements: 1.3, 9.1, 9.2, 9.3_
  - [x] 1.2 Update `folder/folder_server.go` to import and use `pathutil.IsSubPath` instead of the local `isSubPath`
    - Remove the local `isSubPath` function from `folder_server.go`
    - Update `create_folder.go`, `rename_folder.go`, `delete_folder.go` to use `pathutil.IsSubPath`
    - _Requirements: 1.2, 1.3_

- [x] 2. FolderService migration — remove domain separation
  - [x] 2.1 Update `proto/folder/v1/folder.proto` to remove the `domain` field from all request messages
    - Remove `domain` from `CreateFolderRequest`, `RenameFolderRequest`, `DeleteFolderRequest`
    - Renumber fields: `parent_path` → 1, `name` → 2 in `CreateFolderRequest`; `folder_path` → 1, `new_name` → 2 in `RenameFolderRequest`; `folder_path` → 1 in `DeleteFolderRequest`
    - _Requirements: 1.1_
  - [x] 2.2 Regenerate Go code from the updated folder proto
    - Run `buf generate` in the `proto/` directory
    - _Requirements: 1.1_
  - [x] 2.3 Update `folder/create_folder.go` to resolve paths directly under `dataDir` (no domain prefix)
    - Replace `filepath.Join(s.dataDir, req.GetDomain(), req.GetParentPath())` with `filepath.Join(s.dataDir, req.GetParentPath())`
    - Validate against `s.dataDir` using `pathutil.IsSubPath`
    - _Requirements: 1.2, 1.4_
  - [x] 2.4 Update `folder/rename_folder.go` to resolve paths directly under `dataDir`
    - Remove `domainRoot` variable, resolve `oldPath` against `s.dataDir` directly
    - _Requirements: 1.5_
  - [x] 2.5 Update `folder/delete_folder.go` to resolve paths directly under `dataDir`
    - Remove `domainRoot` variable, resolve `target` against `s.dataDir` directly
    - _Requirements: 1.6_
  - [x] 2.6 Update existing folder tests to remove `Domain` field from all request structs
    - Update `create_folder_test.go`, `error_conditions_test.go`, `rename_delete_test.go` to remove `Domain` field usage
    - Tests should create temp dirs without a domain subdirectory
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6_

- [x] 3. Checkpoint — folder migration
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. NotesService adaptation — add `note_` prefix
  - [x] 4.1 Update `server/createNote.go` to create files with `note_` prefix
    - Change filename from `req.Title+".md"` to `"note_"+req.Title+".md"`
    - Strip `note_` prefix when returning the title in the response
    - _Requirements: 2.3, 2.4_
  - [x] 4.2 Update `server/listNotes.go` to filter by `note_` prefix
    - Only include files with `note_` prefix and `.md` extension in the notes list
    - Skip `tasks_*` files and other non-note files
    - Strip `note_` prefix from title in returned `Note` objects
    - Include subdirectories in entries
    - _Requirements: 2.1, 2.2_
  - [x] 4.3 Update `server/getNote.go` to strip `note_` prefix when deriving title
    - _Requirements: 2.1_
  - [x] 4.4 Update existing notes tests to account for the `note_` prefix
    - Update `createNote_test.go`, `listNotes_test.go`, `getNote_test.go`, `deleteNote_test.go`, `updateNote_test.go` to use `note_` prefixed filenames
    - Update `listNotes_property_test.go` to create files with `note_` prefix and verify filtering
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

- [x] 5. Checkpoint — notes adaptation
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Define tasks proto and generate Go code
  - [x] 6.1 Create `proto/tasks/v1/tasks.proto` with the `TasksService` definition
    - Define `TasksService` with `CreateTaskList`, `GetTaskList`, `ListTaskLists`, `UpdateTaskList`, `DeleteTaskList` RPCs
    - Define messages: `Subtask`, `MainTask`, `CreateTaskListRequest/Response`, `GetTaskListRequest/Response`, `ListTaskListsRequest/Response`, `TaskListEntry`, `UpdateTaskListRequest/Response`, `DeleteTaskListRequest/Response`
    - Use `package tasks.v1` and `option go_package = "gen/tasks;tasks"`
    - _Requirements: 10.1, 10.2, 10.3_
  - [x] 6.2 Update `proto/buf.yaml` if needed and run `buf generate` to produce Go code
    - Ensure the generated `proto/gen/tasks/v1/` and `proto/gen/tasks/v1/tasksv1connect/` packages are created
    - _Requirements: 10.1_

- [x] 7. Implement task file parser and printer
  - [x] 7.1 Create `tasks/types.go` with `MainTask` and `Subtask` Go domain types
    - Define `MainTask` struct with `Description`, `Done`, `DueDate`, `Recurrence`, `Subtasks` fields
    - Define `Subtask` struct with `Description`, `Done` fields
    - _Requirements: 4.1, 5.1_
  - [x] 7.2 Create `tasks/printer.go` with `PrintTaskFile(tasks []MainTask) []byte`
    - Main tasks: `- [ ] Description` or `- [x] Description`
    - Deadline tasks append `| due:YYYY-MM-DD`
    - Recurring tasks append `| due:YYYY-MM-DD | recurrence:RRULE_STRING`
    - Subtasks: 2-space indent `  - [ ] Description`
    - _Requirements: 7.2, 7.3, 7.4_
  - [x] 7.3 Create `tasks/parser.go` with `ParseTaskFile(data []byte) ([]MainTask, error)`
    - Parse main tasks at column 0 with `- [ ] ` or `- [x] ` prefix
    - Parse subtasks at 2-space indent with `  - [ ] ` or `  - [x] ` prefix
    - Parse optional metadata after `|` delimiter: `due:YYYY-MM-DD`, `recurrence:RRULE_STRING`
    - Return descriptive parse error with line number on malformed input
    - Ignore blank lines
    - _Requirements: 7.5, 7.6_
  - [x] 7.4 Write property test for parse/print round-trip (`tasks/parser_property_test.go`)
    - **Property 1: Task file parse/print round-trip**
    - Create `mainTaskGen()`, `subtaskGen()`, `taskListGen()` rapid generators
    - For any valid `[]MainTask`, `ParseTaskFile(PrintTaskFile(tasks))` must produce identical tasks
    - Printing the parsed result must produce byte-identical output
    - **Validates: Requirements 7.2, 7.3, 7.4, 7.5, 7.7**
  - [x] 7.5 Write property test for malformed input parse errors (`tasks/parser_property_test.go`)
    - **Property 14: Malformed task file produces parse error with line number**
    - Generate byte sequences that are not valid task files
    - `ParseTaskFile` must return an error containing the line number
    - **Validates: Requirements 7.6**
  - [x] 7.6 Write unit tests for parser edge cases (`tasks/parser_test.go`)
    - Test empty file, single task, task with subtasks, all three modes, metadata parsing
    - Test specific known file content against expected parsed output
    - _Requirements: 7.2, 7.3, 7.4, 7.5, 7.6_

- [ ] 8. Implement RRULE helpers
  - [ ] 8.1 Create `tasks/rrule.go` with `ComputeNextDueDate` and `ValidateRRule` functions
    - `ComputeNextDueDate(rruleStr string, after time.Time) (time.Time, error)` wraps `teambition/rrule-go`
    - `ValidateRRule(rruleStr string) error` checks RFC 5545 conformance
    - Add `github.com/teambition/rrule-go` dependency via `go get`
    - _Requirements: 6.1, 6.2, 6.5_
  - [ ] 8.2 Write unit tests for RRULE helpers (`tasks/rrule_test.go`)
    - Test specific RRULE strings with known expected next dates
    - Test invalid RRULE strings return errors
    - _Requirements: 6.1, 6.5_

- [ ] 9. Checkpoint — parser, printer, and RRULE helpers
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 10. Implement TaskServer RPC methods
  - [ ] 10.1 Create `tasks/task_server.go` with `TaskServer` struct and `NewTaskServer(dataDir string)` constructor
    - Embed `tasksv1connect.UnimplementedTasksServiceHandler`
    - Store `dataDir` field
    - Add proto-to-domain and domain-to-proto conversion helpers
    - _Requirements: 10.1, 10.2_
  - [ ] 10.2 Implement `CreateTaskList` in `tasks/create_task_list.go`
    - Validate path with `pathutil.ValidatePath`, validate name (non-empty, no separators)
    - Validate tasks: reject if any main task has both `due_date` and `recurrence`, reject subtasks with due date or recurrence
    - For recurring tasks, compute first due date via `ComputeNextDueDate`
    - Create intermediate directories with `os.MkdirAll`
    - Check for existing file (return AlreadyExists if present)
    - Write file atomically using `PrintTaskFile` output
    - _Requirements: 3.1, 3.6, 4.2, 4.3, 4.4, 4.5, 5.4, 5.5, 7.1, 7.8, 8.1, 8.2, 9.1, 9.2, 9.3_
  - [ ] 10.3 Implement `GetTaskList` in `tasks/get_task_list.go`
    - Validate path, read file, parse with `ParseTaskFile`, return task list
    - Return NotFound if file doesn't exist
    - _Requirements: 3.2, 3.7, 9.1, 9.2, 9.3_
  - [ ] 10.4 Implement `ListTaskLists` in `tasks/list_task_lists.go`
    - Validate path, read directory, filter for `tasks_` prefixed `.md` files
    - Return `TaskListEntry` for each task file and folder names in entries
    - _Requirements: 3.3_
  - [ ] 10.5 Implement `UpdateTaskList` in `tasks/update_task_list.go`
    - Validate path, validate tasks (same rules as create)
    - For recurring tasks marked done: reset to open, compute next due date
    - Read existing file to compare recurring task state changes
    - Write updated file atomically
    - _Requirements: 3.4, 4.5, 5.4, 5.5, 6.3, 6.4, 7.8, 9.1, 9.2, 9.3_
  - [ ] 10.6 Implement `DeleteTaskList` in `tasks/delete_task_list.go`
    - Validate path, remove file, return NotFound if file doesn't exist
    - _Requirements: 3.5, 3.8, 9.1, 9.2, 9.3_

- [ ] 11. Register TaskServer in `main.go`
  - Import `tasksv1connect` and `tasks` packages
  - Create `tasks.NewTaskServer(dataDir)` and register with `tasksv1connect.NewTasksServiceHandler`
  - Add `"tasks.v1.TasksService"` to the gRPC reflector
  - _Requirements: 10.1, 10.3_

- [ ] 12. Checkpoint — TaskServer wired up
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 13. Property-based tests for NotesService adaptation
  - [ ] 13.1 Write property test: ListNotes excludes non-note files (`server/listNotes_property_test.go`)
    - **Property 2: ListNotes excludes non-note files**
    - Create a directory with a mix of `note_*.md`, `tasks_*.md`, other `.md`, and non-`.md` files
    - `ListNotes` must return only `note_` prefixed `.md` files in notes; entries include those files plus subdirectories
    - **Validates: Requirements 2.1, 2.2**
  - [ ] 13.2 Write property test: Created notes use `note_` prefix (`server/createNote_property_test.go`)
    - **Property 4: Created notes use note_ prefix**
    - For any valid title and content, after `CreateNote`, the file on disk must be named `note_<title>.md`
    - **Validates: Requirements 2.3, 2.4**

- [ ] 14. Property-based tests for TaskServer
  - [ ] 14.1 Write property test: Created task lists use `tasks_` prefix (`tasks/task_server_property_test.go`)
    - **Property 5: Created task lists use tasks_ prefix**
    - For any valid name, after `CreateTaskList`, the file on disk must be named `tasks_<name>.md`
    - **Validates: Requirements 3.1, 7.1**
  - [ ] 14.2 Write property test: Task list create-then-get round-trip (`tasks/task_server_property_test.go`)
    - **Property 6: Task list create-then-get round-trip**
    - Create a task list, then `GetTaskList` — returned tasks must match the input
    - **Validates: Requirements 3.2, 3.4**
  - [ ] 14.3 Write property test: Duplicate name returns already-exists (`tasks/task_server_property_test.go`)
    - **Property 7: Duplicate task list name returns already-exists**
    - Create a task list, then create another with the same name in the same folder — must fail with AlreadyExists
    - **Validates: Requirements 3.6**
  - [ ] 14.4 Write property test: Non-existent paths return not-found (`tasks/task_server_property_test.go`)
    - **Property 8: Operations on non-existent paths return not-found**
    - `GetTaskList` and `DeleteTaskList` on non-existent paths must return NotFound
    - **Validates: Requirements 3.7, 3.8**
  - [ ] 14.5 Write property test: Mutual exclusion of due date and recurrence (`tasks/task_server_property_test.go`)
    - **Property 9: Mutual exclusion of due date and recurrence on main tasks**
    - Tasks with both `due_date` and `recurrence` set must be rejected with InvalidArgument
    - **Validates: Requirements 4.5**
  - [ ] 14.6 Write property test: Valid RRULE produces computed due date (`tasks/task_server_property_test.go`)
    - **Property 10: Valid RRULE produces a computed due date**
    - Create a recurring task — returned `due_date` must be non-empty and on or after current date
    - Create `validRRuleGen()` generator for supported RRULE subset
    - **Validates: Requirements 4.3, 6.1, 6.2**
  - [ ] 14.7 Write property test: Recurring task done-advance cycle (`tasks/task_server_property_test.go`)
    - **Property 11: Recurring task done-advance cycle**
    - Mark a recurring task done via `UpdateTaskList` — returned task must be `done=false` with a later due date
    - **Validates: Requirements 6.3, 6.4**
  - [ ] 14.8 Write property test: Invalid RRULE rejected (`tasks/task_server_property_test.go`)
    - **Property 12: Invalid RRULE rejected**
    - Tasks with invalid RRULE strings must be rejected with InvalidArgument
    - Create `invalidRRuleGen()` generator
    - **Validates: Requirements 6.5**
  - [ ] 14.9 Write property test: Path traversal prevention (`tasks/task_server_property_test.go`)
    - **Property 13: Path traversal prevention**
    - All five RPC methods must reject paths with `..` or paths resolving outside data directory
    - Create `traversalPathGen()` generator
    - **Validates: Requirements 1.3, 9.1, 9.2, 9.3**
  - [ ] 14.10 Write property test: ListTaskLists excludes non-task files (`tasks/task_server_property_test.go`)
    - **Property 3: ListTaskLists excludes non-task files**
    - Directory with mixed files — `ListTaskLists` returns only `tasks_` prefixed `.md` files
    - **Validates: Requirements 3.3**
  - [ ] 14.11 Write property test: Auto-create folders on task list creation (`tasks/task_server_property_test.go`)
    - **Property 15: Auto-create folders on task list creation**
    - `CreateTaskList` with non-existent intermediate directories must succeed and create them
    - **Validates: Requirements 8.2**
  - [ ] 14.12 Write property test: Delete removes task list from disk (`tasks/task_server_property_test.go`)
    - **Property 16: Delete removes task list from disk**
    - After `DeleteTaskList`, the file must not exist and `GetTaskList` must return NotFound
    - **Validates: Requirements 3.5**

- [ ] 15. Final checkpoint
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation after each major phase
- Property tests validate the 16 correctness properties defined in the design document
- The dependency order is: pathutil → folder migration → notes adaptation → proto + codegen → parser/printer + RRULE → TaskServer → main.go registration → property tests
