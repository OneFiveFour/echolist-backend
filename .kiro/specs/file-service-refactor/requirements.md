# Requirements Document

## Introduction

Refactor the existing `FolderService` Connect-RPC service into a general-purpose `FileService`. The new service focuses on file-system CRUD operations: creating folders, renaming folders, deleting folders, and listing the immediate contents (both files and folders) of a given directory. The `GetFolder` RPC is removed as redundant. Notes and tasks creation remain in their respective services. All proto definitions, generated code, Go implementation files, and wiring in `main.go` must be updated to reflect the rename.

## Glossary

- **File_Service**: The renamed Connect-RPC service (previously `FolderService`) responsible for file-system-level CRUD operations.
- **File_Server**: The Go struct implementing the `File_Service` RPC interface (previously `FolderServer`).
- **Data_Directory**: The root directory on disk that the backend manages; all paths are resolved relative to this directory.
- **Parent_Path**: A relative path (from the Data_Directory root) identifying the directory whose contents are being queried or modified.
- **File_Entry**: A string representing a single item inside a directory. Folder entries end with a trailing `/`; file entries do not.
- **Note_Service**: The existing Connect-RPC service responsible for creating, reading, updating, and deleting notes.
- **Task_Service**: The existing Connect-RPC service responsible for creating, reading, updating, and deleting task lists.

## Requirements

### Requirement 1: Rename FolderService to FileService

**User Story:** As a developer, I want the folder service renamed to a file service, so that the API reflects its broader file-system scope.

#### Acceptance Criteria

1. THE File_Service SHALL be defined in a new proto file at `proto/file/v1/file.proto` with package `file.v1`.
2. THE File_Service SHALL expose the RPCs: `CreateFolder`, `ListFiles`, `UpdateFolder`, and `DeleteFolder`.
3. WHEN the proto file is compiled, THE generated Go code SHALL be placed under `proto/gen/file/v1/` and `proto/gen/file/v1/filev1connect/`.
4. THE File_Server Go implementation SHALL reside in a `file/` package directory, replacing the `folder/` package.
5. THE `main.go` file SHALL register the File_Service handler using the generated `filev1connect` package and update the gRPC reflection service name to `file.v1.FileService`.
6. THE old `proto/folder/` directory, `proto/gen/folder/` directory, and `folder/` Go package SHALL be removed after the migration is complete.

### Requirement 2: Remove GetFolder RPC

**User Story:** As a developer, I want the GetFolder RPC removed, so that the API does not expose a redundant operation that returns already-known information.

#### Acceptance Criteria

1. THE File_Service proto definition SHALL NOT include a `GetFolder` RPC.
2. THE File_Server Go implementation SHALL NOT contain a `GetFolder` method.
3. WHEN a client calls a removed `GetFolder` endpoint, THE File_Service SHALL return an `Unimplemented` error (default Connect-RPC behavior for undefined RPCs).

### Requirement 3: ListFiles RPC

**User Story:** As a frontend developer, I want to list all immediate children (files and folders) of a given directory, so that I can render folder contents or incrementally build a file tree UI.

#### Acceptance Criteria

1. WHEN a valid Parent_Path is provided, THE File_Service `ListFiles` RPC SHALL return a list of File_Entry strings representing all immediate children of that directory.
2. THE File_Service `ListFiles` RPC SHALL represent folder entries with a trailing `/` suffix and file entries without a trailing `/` suffix.
3. THE File_Service `ListFiles` RPC SHALL return only immediate children of the requested directory (non-recursive).
4. WHEN the Parent_Path is empty or root, THE File_Service `ListFiles` RPC SHALL return the immediate children of the Data_Directory.
5. IF the Parent_Path does not exist, THEN THE File_Service SHALL return a `NotFound` error.
6. IF the Parent_Path refers to a file instead of a directory, THEN THE File_Service SHALL return a `NotFound` error with a message indicating the path is not a directory.
7. IF the Parent_Path escapes the Data_Directory, THEN THE File_Service SHALL return an `InvalidArgument` error.

### Requirement 4: Existing Folder RPCs preserved

**User Story:** As a developer, I want the existing CreateFolder, UpdateFolder, and DeleteFolder operations to continue working under the new File_Service, so that no functionality is lost during the rename.

#### Acceptance Criteria

1. THE File_Service SHALL expose `CreateFolder`, `UpdateFolder`, and `DeleteFolder` RPCs with the same request/response messages and behavior as the existing FolderService.
2. THE existing Go implementations for these RPCs SHALL be migrated to the new `file/` package with updated import paths only; no logic changes are required.

### Requirement 5: Notes and Tasks creation remains unchanged

**User Story:** As a developer, I want note and task creation to remain in their respective services, so that domain-specific logic stays separated from file-system operations.

#### Acceptance Criteria

1. THE Note_Service SHALL continue to handle note creation via its `CreateNote` RPC without changes.
2. THE Task_Service SHALL continue to handle task list creation via its `CreateTaskList` RPC without changes.
3. THE File_Service SHALL NOT include any RPCs for creating notes or task lists.
