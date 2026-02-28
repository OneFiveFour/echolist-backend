# Requirements Document

## Introduction

This spec addresses six issues found during a code review of the echolist-backend project. The issues span a bug in the file listing filter, an API inconsistency in the notes proto, missing path traversal protection in the NoteService, inconsistent error handling, missing input validation, and duplicate utility code. The goal is to harden the API surface, align the NoteService with the patterns already established in the TaskService, and eliminate code duplication.

## Glossary

- **FileService**: The Connect/gRPC service that handles directory browsing and file listing (`file/list_files.go`).
- **NoteService**: The Connect/gRPC service that handles CRUD operations for notes (`server/` package).
- **TaskService**: The Connect/gRPC service that handles CRUD operations for task lists (`tasks/` package). Used as the reference implementation for correct patterns.
- **ListFiles**: The FileService RPC that lists files and directories under a given parent path.
- **ListNotes**: The NoteService RPC that lists notes under a given path.
- **pathutil**: The shared utility package providing `ValidatePath` and `IsSubPath` functions for path traversal protection.
- **atomicWriteFile**: A utility function that writes data to a file atomically via a temp file and rename.
- **Connect_Error_Code**: A structured error code from the Connect RPC framework (e.g., `CodeNotFound`, `CodeInternal`, `CodeInvalidArgument`).
- **Path_Traversal**: An attack where a malicious relative path (e.g., `../../etc/passwd`) escapes the intended data directory.

## Requirements

### Requirement 1: Fix File Listing Task File Filter

**User Story:** As a client of the FileService, I want ListFiles to correctly return task list files, so that I can browse all content in a directory.

#### Acceptance Criteria

1. WHEN listing files in a directory containing task list files, THE ListFiles handler SHALL include files with the `tasks_` prefix in the results.
2. WHEN listing files in a directory containing note files, THE ListFiles handler SHALL include files with the `note_` prefix in the results.
3. WHEN listing files in a directory containing files that match neither the `note_` nor the `tasks_` prefix, THE ListFiles handler SHALL exclude those files from the results.

### Requirement 2: Remove Entries Field from ListNotesResponse

**User Story:** As a maintainer of the API, I want the ListNotesResponse to only return notes, so that directory browsing responsibility belongs solely to the FileService.

#### Acceptance Criteria

1. THE ListNotesResponse proto message SHALL contain only the `notes` field and SHALL NOT contain an `entries` field.
2. WHEN the ListNotes RPC is called, THE NoteService SHALL return only Note objects in the response.
3. WHEN the ListNotes RPC is called on a directory containing subdirectories, THE NoteService SHALL exclude subdirectory entries from the response.
4. WHEN the ListNotesResponse proto is regenerated, THE ListNotes handler and all associated tests SHALL compile and pass without referencing the removed `entries` field.

### Requirement 3: Add Path Traversal Protection to NoteService

**User Story:** As a system administrator, I want the NoteService to reject path traversal attempts, so that clients cannot read or write files outside the data directory.

#### Acceptance Criteria

1. WHEN a CreateNote request contains a `path` that would resolve outside the data directory, THE NoteService SHALL return a Connect error with code `CodeInvalidArgument`.
2. WHEN a GetNote request contains a `file_path` that would resolve outside the data directory, THE NoteService SHALL return a Connect error with code `CodeInvalidArgument`.
3. WHEN an UpdateNote request contains a `file_path` that would resolve outside the data directory, THE NoteService SHALL return a Connect error with code `CodeInvalidArgument`.
4. WHEN a DeleteNote request contains a `file_path` that would resolve outside the data directory, THE NoteService SHALL return a Connect error with code `CodeInvalidArgument`.
5. THE NoteService SHALL use the `pathutil.ValidatePath` or `pathutil.IsSubPath` functions for all path validation, consistent with the TaskService implementation.

### Requirement 4: Use Proper Connect Error Codes in NoteService

**User Story:** As a client of the NoteService, I want to receive structured Connect error codes, so that I can programmatically distinguish between different failure modes.

#### Acceptance Criteria

1. WHEN a GetNote request references a file that does not exist, THE NoteService SHALL return a Connect error with code `CodeNotFound`.
2. WHEN a DeleteNote request references a file that does not exist, THE NoteService SHALL return a Connect error with code `CodeNotFound`.
3. IF a file system read operation fails in any NoteService handler, THEN THE NoteService SHALL return a Connect error with code `CodeInternal`.
4. IF a file system write operation fails in any NoteService handler, THEN THE NoteService SHALL return a Connect error with code `CodeInternal`.
5. THE NoteService SHALL NOT return raw Go errors (e.g., from `os.Stat`, `os.ReadFile`, `os.Remove`) directly to clients.

### Requirement 5: Add Input Validation to CreateNote

**User Story:** As a client of the NoteService, I want CreateNote to validate inputs, so that malformed requests are rejected with clear error messages.

#### Acceptance Criteria

1. WHEN a CreateNote request has an empty `title`, THE NoteService SHALL return a Connect error with code `CodeInvalidArgument` and a message indicating the title is required.
2. WHEN a CreateNote request has a `title` containing path separator characters (`/` or `\`), THE NoteService SHALL return a Connect error with code `CodeInvalidArgument` and a message indicating the title must not contain path separators.

### Requirement 6: Extract Shared atomicWriteFile to a Common Package

**User Story:** As a maintainer of the codebase, I want a single shared implementation of atomicWriteFile, so that bug fixes and improvements apply everywhere.

#### Acceptance Criteria

1. THE codebase SHALL contain exactly one implementation of the `atomicWriteFile` function, located in a shared utility package.
2. THE NoteService SHALL use the shared `atomicWriteFile` function for all file write operations.
3. THE TaskService SHALL use the shared `atomicWriteFile` function for all file write operations.
4. WHEN the shared `atomicWriteFile` function is introduced, THE existing tests for both the NoteService and TaskService SHALL continue to pass.
