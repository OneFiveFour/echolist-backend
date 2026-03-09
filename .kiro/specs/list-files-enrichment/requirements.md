# Requirements Document

## Introduction

The FileService's ListFiles endpoint currently returns a flat list of path strings. This feature enriches the response so each entry carries structured metadata: an item type discriminator (FOLDER, NOTE, TASK_LIST), a title, and type-specific metadata. Folders carry a title and child count. Notes carry a title, updated_at timestamp, and a content preview (first 100 characters). Task lists carry a title, updated_at timestamp, total MainTask count, and finished MainTask count. The goal is to give the frontend everything it needs to render a unified file-tree browser in a single RPC call.

## Glossary

- **FileService**: The gRPC/Connect service defined in `proto/file/v1/file.proto` that manages folder and file operations.
- **ListFiles_RPC**: The `ListFiles` RPC method on the FileService that returns directory contents for a given parent path.
- **FileEntry**: A protobuf message representing a single item returned by the ListFiles_RPC, replacing the former plain string entry.
- **ItemType**: A protobuf enum with values FOLDER, NOTE, and TASK_LIST that classifies each FileEntry.
- **NoteFileType**: The file naming convention defined in `common/pathutil.go` with prefix `note_` and suffix `.md`.
- **TaskListFileType**: The file naming convention defined in `common/pathutil.go` with prefix `tasks_` and suffix `.md`.
- **Child_Count**: The number of immediate children (files and subdirectories) inside a folder.
- **Content_Preview**: The first 100 characters of a note's content, used for display in the file tree.
- **Total_Task_Count**: The number of MainTask items in a task list.
- **Done_Task_Count**: The number of MainTask items in a task list whose `done` field is true.
- **Parent_Dir**: The directory path passed in the ListFiles request, relative to the data root.

## Requirements

### Requirement 1: Structured FileEntry Response

**User Story:** As a frontend developer, I want each ListFiles result item to be a structured message instead of a plain string, so that I can render a rich file-tree UI without additional parsing.

#### Acceptance Criteria

1. THE ListFiles_RPC SHALL return a list of FileEntry messages instead of a list of strings.
2. THE FileEntry message SHALL contain a `path` field holding the relative path of the item within the Parent_Dir.
3. THE FileEntry message SHALL contain a `title` field holding the human-readable display title of the item.
4. THE FileEntry message SHALL contain an `item_type` field of type ItemType.

### Requirement 2: Item Type Classification

**User Story:** As a frontend developer, I want each entry to carry a type discriminator, so that I can render folders, notes, and task lists with distinct icons and behaviors.

#### Acceptance Criteria

1. THE ItemType enum SHALL define exactly three values: FOLDER, NOTE, and TASK_LIST.
2. WHEN a directory entry is encountered, THE ListFiles_RPC SHALL classify the FileEntry as ItemType FOLDER.
3. WHEN a file matching the NoteFileType naming convention is encountered, THE ListFiles_RPC SHALL classify the FileEntry as ItemType NOTE.
4. WHEN a file matching the TaskListFileType naming convention is encountered, THE ListFiles_RPC SHALL classify the FileEntry as ItemType TASK_LIST.
5. WHEN a file matches neither NoteFileType nor TaskListFileType, THE ListFiles_RPC SHALL exclude the file from the response.

### Requirement 3: Folder Metadata

**User Story:** As a frontend developer, I want to know the title and how many items a folder contains, so that I can show a label, count badge, or decide whether to render an expand arrow.

#### Acceptance Criteria

1. WHEN a FileEntry has ItemType FOLDER, THE ListFiles_RPC SHALL set the `title` field to the directory name.
2. WHEN a FileEntry has ItemType FOLDER, THE ListFiles_RPC SHALL populate a `child_count` field with the total number of immediate children that are directories, notes, or task lists.
3. THE `child_count` SHALL only be zero when the folder contains no directories, no notes, and no task lists (i.e. it is empty or contains only unrecognized files).

### Requirement 4: Note Metadata

**User Story:** As a frontend developer, I want to see the title, last-updated timestamp, and a content preview for notes in the directory listing, so that I can display meaningful labels, sort by recency, and show a snippet without fetching full content.

#### Acceptance Criteria

1. WHEN a FileEntry has ItemType NOTE, THE ListFiles_RPC SHALL populate a `title` field extracted from the filename using the NoteFileType prefix and suffix convention.
2. WHEN a FileEntry has ItemType NOTE, THE ListFiles_RPC SHALL populate an `updated_at` field with the file's last modification time in Unix milliseconds.
3. WHEN a FileEntry has ItemType NOTE, THE ListFiles_RPC SHALL populate a `preview` field with the first 100 characters of the note's content.
4. WHEN the note content is shorter than 100 characters, THE ListFiles_RPC SHALL set the `preview` field to the full content.

### Requirement 5: Task List Metadata

**User Story:** As a frontend developer, I want to see the title, last-updated timestamp, and task completion progress for task lists in the directory listing, so that I can display a progress indicator without fetching the full task list.

#### Acceptance Criteria

1. WHEN a FileEntry has ItemType TASK_LIST, THE ListFiles_RPC SHALL populate a `title` field extracted from the filename using the TaskListFileType prefix and suffix convention.
2. WHEN a FileEntry has ItemType TASK_LIST, THE ListFiles_RPC SHALL populate an `updated_at` field with the file's last modification time in Unix milliseconds.
3. WHEN a FileEntry has ItemType TASK_LIST, THE ListFiles_RPC SHALL populate a `total_task_count` field with the number of MainTask items in the task list.
4. WHEN a FileEntry has ItemType TASK_LIST, THE ListFiles_RPC SHALL populate a `done_task_count` field with the number of MainTask items whose `done` field is true.

### Requirement 6: Path Format Consistency

**User Story:** As a frontend developer, I want paths in the response to follow a consistent format, so that I can use them directly in subsequent GetNote, GetTaskList, or ListFiles calls.

#### Acceptance Criteria

1. WHEN the Parent_Dir in the request is non-empty, THE ListFiles_RPC SHALL set each FileEntry path to `{Parent_Dir}/{entry_filename}` (e.g. `ListFiles(parent_dir: "Work")` returns a note with path `Work/note_Meeting.md`).
2. WHEN the Parent_Dir in the request is empty, THE ListFiles_RPC SHALL set each FileEntry path to the filesystem entry name only (e.g. `ListFiles(parent_dir: "")` returns a folder with path `Projects`).
3. THE ListFiles_RPC SHALL produce paths that are directly usable as `file_path` in GetNote and GetTaskList, or as `parent_dir` in nested ListFiles calls, without modification.

### Requirement 7: Error Handling

**User Story:** As a backend developer, I want the enriched ListFiles to validate and sanitize inputs using the existing common package utilities, so that error behavior is consistent across all services.

#### Acceptance Criteria

1. THE ListFiles_RPC SHALL use `common.ValidateParentDir` to validate and sanitize the Parent_Dir input, which handles path-traversal prevention and root-path allowance.
2. THE ListFiles_RPC SHALL use `common.RequireDir` to verify the resolved path is an existing directory.
3. IF `common.ValidateParentDir` rejects the input, THEN THE ListFiles_RPC SHALL propagate the ConnectRPC error returned by that function.
4. IF `common.RequireDir` rejects the input, THEN THE ListFiles_RPC SHALL propagate the ConnectRPC error returned by that function.
5. IF the underlying directory read fails for an I/O reason, THEN THE ListFiles_RPC SHALL return a ConnectRPC Internal error.

### Requirement 8: Empty Directory Handling

**User Story:** As a frontend developer, I want an empty directory to return an empty list of FileEntry messages, so that I can render an empty-state UI.

#### Acceptance Criteria

1. WHEN the Parent_Dir is a valid directory containing no children, THE ListFiles_RPC SHALL return an empty list of FileEntry messages.
2. WHEN the Parent_Dir contains only files that match neither NoteFileType nor TaskListFileType, THE ListFiles_RPC SHALL return an empty list of FileEntry messages.

### Requirement 9: Non-Recursive Listing

**User Story:** As a backend developer, I want ListFiles to only return immediate children, so that performance remains predictable and the frontend controls tree expansion.

#### Acceptance Criteria

1. THE ListFiles_RPC SHALL return only immediate children of the Parent_Dir.
2. THE ListFiles_RPC SHALL not recurse into subdirectories.
