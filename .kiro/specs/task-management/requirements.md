# Requirements Document

## Introduction

Task management feature for the echolist-backend. Users can create, read, update, and delete task lists organized in folders. The system uses a single unified `data` directory where both notes and task lists are stored as `.md` files, distinguished by filename prefix: notes use a `note_` prefix (e.g., `note_meeting-notes.md`) and task lists use a `tasks_` prefix (e.g., `tasks_groceries.md`). The user decides how to organize notes and tasks — there is no forced separation into domain-specific subdirectories. This file-based approach enables simple backups by copying the data folder.

Tasks support a main-task/subtask hierarchy (one level deep), open/done status, and one of three modes for main tasks:

1. **Simple task** — description and status only; no due date, no recurrence.
2. **Deadline task** — description, status, and a user-provided due date; no recurrence. Used for "I have to do that by XXX" tasks.
3. **Recurring task** — description, status, and an RRULE recurrence pattern; the server computes the due date from the pattern (never user-provided).

A main task cannot have both a user-provided due date and a recurrence pattern. Recurring tasks use the iCalendar RRULE format (RFC 5545). The backend exposes a Connect-Go (protobuf) RPC API consistent with the existing notes and folder services.

## Glossary

- **Task_Service**: The Connect-Go RPC service that handles all task-related API operations (create, read, update, delete task lists)
- **Task_List**: A single `.md` file with a `tasks_` filename prefix (e.g., `tasks_groceries.md`) stored anywhere in the Data_Directory tree, containing zero or more main tasks
- **Main_Task**: A top-level task entry within a task list; can be open or done, can have subtasks, and operates in one of three modes: simple (no due date, no recurrence), deadline (user-provided due date, no recurrence), or recurring (RRULE recurrence pattern, server-computed due date). A main task cannot have both a user-provided due date and a recurrence pattern.
- **Deadline_Task**: A main task with a user-provided due date and no recurrence pattern, representing a one-off deadline ("I have to do that by XXX")
- **Subtask**: A child task nested under a main task; can be open or done but cannot have its own subtasks, cannot be recurring, and cannot have a due date
- **Task_File**: The human-readable markdown-like text file on disk (with `tasks_` prefix and `.md` extension) that persists a task list, using checkbox syntax (`[ ]` for open, `[x]` for done)
- **Data_Directory**: The single root directory for all persistent data. Notes (`note_` prefixed `.md` files) and task lists (`tasks_` prefixed `.md` files) coexist in the same folder hierarchy with no domain-based separation.
- **Folder_Service**: The existing Connect-Go RPC service that handles folder CRUD operations (create, rename, delete) directly under the Data_Directory
- **Notes_Service**: The existing Connect-Go RPC service that handles note CRUD operations. Notes are identified by the `note_` filename prefix and `.md` extension.
- **RRULE**: A recurrence rule following the iCalendar RRULE format defined in RFC 5545 (e.g., `FREQ=DAILY`, `FREQ=WEEKLY;BYDAY=MO`, `FREQ=MONTHLY;BYDAY=1MO`, `FREQ=DAILY;INTERVAL=3`)
- **Task_File_Parser**: The component that reads a task file from disk and produces an in-memory task list representation
- **Task_File_Printer**: The component that serializes an in-memory task list back into the human-readable task file format

## Requirements

### Requirement 1: FolderService Migration — Remove Domain Separation

**User Story:** As a developer, I want the FolderService to operate directly under the data root without domain-based subdirectories, so that notes and task lists can coexist in the same folder hierarchy.

#### Acceptance Criteria

1. THE Folder_Service SHALL remove the `domain` field from CreateFolderRequest, RenameFolderRequest, and DeleteFolderRequest in `proto/folder/v1/folder.proto`
2. THE Folder_Service SHALL resolve all folder paths directly under the Data_Directory without appending a domain prefix
3. THE Folder_Service SHALL validate that folder paths do not escape the Data_Directory root
4. WHEN a CreateFolder request is received, THE Folder_Service SHALL join the Data_Directory with the parent path and folder name (without any domain segment)
5. WHEN a RenameFolder request is received, THE Folder_Service SHALL resolve the folder path directly under the Data_Directory
6. WHEN a DeleteFolder request is received, THE Folder_Service SHALL resolve the folder path directly under the Data_Directory

### Requirement 2: NotesService Adaptation for Unified Structure

**User Story:** As a user, I want the notes service to continue working correctly in the unified folder structure, so that notes and task lists can coexist without interference.

#### Acceptance Criteria

1. THE Notes_Service SHALL identify notes by the `note_` filename prefix and `.md` extension within the Data_Directory
2. WHEN a ListNotes request is received, THE Notes_Service SHALL return only files with the `note_` prefix (excluding `tasks_` prefixed files and other non-note files) in the specified folder
3. THE Notes_Service SHALL continue to create, read, update, and delete notes as `note_` prefixed `.md` files in any folder under the Data_Directory
4. THE Notes_Service SHALL use the `note_` prefix for all newly created notes

### Requirement 3: Task List CRUD via API

**User Story:** As a user, I want to create, read, update, and delete task lists through the API, so that I can manage my tasks from any client.

#### Acceptance Criteria

1. WHEN a CreateTaskList request is received with a valid name and folder path, THE Task_Service SHALL create a new `.md` file with a `tasks_` prefix in the specified folder under the Data_Directory and return the created task list
2. WHEN a GetTaskList request is received with a valid file path, THE Task_Service SHALL read the task file from disk and return the parsed task list
3. WHEN a ListTaskLists request is received with a folder path, THE Task_Service SHALL return all `tasks_` prefixed `.md` files and subfolders in that folder (excluding `note_` prefixed files and other non-task files)
4. WHEN an UpdateTaskList request is received with a valid file path and updated task data, THE Task_Service SHALL write the updated task list to disk atomically and return the updated task list
5. WHEN a DeleteTaskList request is received with a valid file path, THE Task_Service SHALL remove the task file from disk and return a confirmation
6. IF a CreateTaskList request specifies a name that already exists in the target folder, THEN THE Task_Service SHALL return an "already exists" error
7. IF a GetTaskList request specifies a file path that does not exist, THEN THE Task_Service SHALL return a "not found" error
8. IF a DeleteTaskList request specifies a file path that does not exist, THEN THE Task_Service SHALL return a "not found" error

### Requirement 4: Main Task Structure

**User Story:** As a user, I want to create main tasks in one of three modes — simple, deadline, or recurring — so that I can track work items with the appropriate level of time sensitivity.

#### Acceptance Criteria

1. THE Task_Service SHALL support main tasks with the following fields: description (plain text), status (open or done), optional user-provided due date, and optional RRULE recurrence pattern (from which the server computes the due date)
2. WHEN a main task is created without a due date and without a recurrence pattern, THE Task_Service SHALL store the main task as a simple task with no due date and no recurrence pattern
3. WHEN a main task is created with a recurrence pattern, THE Task_Service SHALL compute the first due date from the RRULE pattern and store the main task with the computed due date
4. WHEN a main task is created with a user-provided due date and without a recurrence pattern, THE Task_Service SHALL store the main task as a deadline task with the provided due date
5. IF a request provides both a user-provided due date and a recurrence pattern on a main task, THEN THE Task_Service SHALL return an "invalid argument" error

### Requirement 5: Subtask Structure

**User Story:** As a user, I want to add subtasks to a main task, so that I can break down work into smaller steps.

#### Acceptance Criteria

1. THE Task_Service SHALL support subtasks nested under a main task, each with a description (plain text) and status (open or done)
2. THE Task_Service SHALL limit subtask nesting to one level (a subtask cannot have its own subtasks)
3. IF a request attempts to add a subtask to a subtask, THEN THE Task_Service SHALL return an "invalid argument" error
4. THE Task_Service SHALL not allow a recurrence pattern or due date on a subtask
5. IF a request attempts to set a recurrence pattern or due date on a subtask, THEN THE Task_Service SHALL return an "invalid argument" error

### Requirement 6: Recurring Task Behavior

**User Story:** As a user, I want recurring tasks to automatically advance their due date when marked done, so that I do not have to manually recreate them.

#### Acceptance Criteria

1. THE Task_Service SHALL support recurrence patterns expressed as iCalendar RRULE strings (RFC 5545), including but not limited to: `FREQ=DAILY`, `FREQ=WEEKLY`, `FREQ=MONTHLY`, `FREQ=YEARLY`, `FREQ=DAILY;INTERVAL=N` (every N days), `FREQ=WEEKLY;BYDAY=MO` (specific weekday), and `FREQ=MONTHLY;BYDAY=1MO` (first Monday of the month)
2. WHEN a recurring main task is created, THE Task_Service SHALL compute the first due date from the RRULE pattern and store the computed due date alongside the task
3. WHEN a recurring main task is marked as done, THE Task_Service SHALL reset the task status to open and compute the next due date from the RRULE pattern
4. WHEN a recurring task's due date is advanced, THE Task_Service SHALL persist the new computed due date to the task file
5. IF a request provides an RRULE string that does not conform to RFC 5545 syntax, THEN THE Task_Service SHALL return an "invalid argument" error

### Requirement 7: Task File Format and Persistence

**User Story:** As a user, I want task lists stored as human-readable markdown-like text files with checkbox syntax and a `tasks_` filename prefix (e.g., `tasks_groceries.md`), so that I can read and understand them without special tools, and they can be distinguished from notes (`note_` prefixed `.md` files) in the same directory.

#### Acceptance Criteria

1. THE Task_Service SHALL use the `tasks_` filename prefix with the `.md` extension for all task list files to distinguish them from notes (`note_` prefixed `.md` files) in the unified folder structure
2. THE Task_File_Printer SHALL serialize each task list into a human-readable markdown-like text format using checkbox syntax (`- [ ]` for open, `- [x]` for done)
3. THE Task_File_Printer SHALL represent each main task on its own line with a checkbox status indicator, description, and optional metadata (due date and/or RRULE) appended after a `|` delimiter (e.g., `- [ ] Buy milk | due:2025-01-15 | recurrence:FREQ=WEEKLY;BYDAY=MO` for a recurring task, `- [ ] Submit report | due:2025-02-28` for a deadline task, `- [ ] Buy groceries` for a simple task)
4. THE Task_File_Printer SHALL represent each subtask indented under its parent main task with a checkbox status indicator and description (e.g., `  - [ ] Whole milk 2L`)
5. THE Task_File_Parser SHALL parse a task file back into an in-memory task list representation
6. IF the Task_File_Parser encounters a malformed task file, THEN THE Task_File_Parser SHALL return a descriptive parse error indicating the line number and nature of the problem
7. FOR ALL valid task lists, parsing a printed task file and then printing the result SHALL produce an identical file (round-trip property)
8. THE Task_Service SHALL write task files atomically to prevent data corruption from interrupted writes

### Requirement 8: Folder Organization for Tasks

**User Story:** As a user, I want to organize my task lists into folders alongside my notes, so that I can group related content together in a way that makes sense to me.

#### Acceptance Criteria

1. THE Task_Service SHALL support creating task lists inside any folder within the Data_Directory, alongside notes and other files
2. WHEN a CreateTaskList request specifies a folder path that does not exist, THE Task_Service SHALL create the necessary folders before creating the task file
3. THE Task_Service SHALL reuse the existing Folder_Service for folder CRUD operations (the Folder_Service operates directly under the Data_Directory without domain separation)

### Requirement 9: Path Traversal Prevention

**User Story:** As a user, I want the server to reject malicious file paths, so that the system remains secure.

#### Acceptance Criteria

1. WHEN a request contains a file path that resolves outside the Data_Directory, THE Task_Service SHALL return an "invalid argument" error
2. WHEN a request contains a file path with path traversal sequences (e.g., `..`), THE Task_Service SHALL reject the request with an "invalid argument" error
3. THE Task_Service SHALL validate all file path inputs against the single Data_Directory root before performing any file system operation

### Requirement 10: Protobuf Service Definition

**User Story:** As a developer, I want a protobuf service definition for tasks, so that the API is consistent with the existing notes and folder services and clients can be generated automatically.

#### Acceptance Criteria

1. THE Task_Service SHALL be defined as a protobuf service under `proto/tasks/v1/tasks.proto`
2. THE Task_Service SHALL follow the same package and option conventions as the existing notes and folder proto definitions
3. THE Task_Service SHALL expose RPC methods for CreateTaskList, GetTaskList, ListTaskLists, UpdateTaskList, and DeleteTaskList
