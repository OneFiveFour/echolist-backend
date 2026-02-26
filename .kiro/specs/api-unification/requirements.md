# Requirements Document

## Introduction

Unify and simplify the public Connect/gRPC API of the echolist backend. The current API has inconsistent service naming (plural vs singular), inconsistent response payloads (some mutations return sibling listings, some return partial fields, some return the full resource), and missing CRUD operations for folders. This spec standardizes all three domain services (NoteService, FolderService, TaskListService) to follow a uniform naming scheme (CreateXxx, GetXxx, ListXxxs, UpdateXxx, DeleteXxx) with consistent response patterns: mutations return the affected resource, and deletes return an empty response.

## Glossary

- **Backend**: The Go server application exposing Connect/gRPC services defined via protobuf
- **NoteService**: The renamed service (from NotesService) responsible for note CRUD operations
- **FolderService**: The existing service responsible for folder CRUD operations, extended with missing RPCs
- **TaskListService**: The renamed service (from TasksService) responsible for task list CRUD operations
- **Note**: A protobuf message representing a note resource with file_path, title, content, and updated_at fields
- **Folder**: A protobuf message representing a folder resource with path and name fields
- **TaskList**: A protobuf message representing a task list resource with file_path, name, tasks, and updated_at fields
- **Affected_Resource**: The single domain resource that was created, read, or modified by an RPC call
- **Empty_Response**: A protobuf response message with no fields, used exclusively for delete operations
- **Connect_Handler**: The generated Go interface and registration function produced by the Connect framework from a protobuf service definition
- **Reflector**: The gRPC reflection configuration in main.go that advertises available services

## Requirements

### Requirement 1: Rename NotesService to NoteService

**User Story:** As a developer, I want the notes service to use the singular name NoteService, so that all service names follow a consistent `<Domain>Service` convention.

#### Acceptance Criteria

1. THE Backend SHALL define a protobuf service named `NoteService` in `proto/notes/v1/notes.proto` replacing the current `NotesService` definition
2. THE Backend SHALL regenerate Connect_Handler code so that the generated Go package exposes `NoteServiceHandler`, `UnimplementedNoteServiceHandler`, and `NewNoteServiceHandler` symbols
3. THE Backend SHALL update the Go server struct to embed `UnimplementedNoteServiceHandler` instead of `UnimplementedNotesServiceHandler`
4. THE Backend SHALL update main.go to call `NewNoteServiceHandler` and register the Reflector with the service name `notes.v1.NoteService`

### Requirement 2: Rename TasksService to TaskListService

**User Story:** As a developer, I want the tasks service to use the name TaskListService matching its domain resource, so that the service name clearly reflects the resource it manages.

#### Acceptance Criteria

1. THE Backend SHALL define a protobuf service named `TaskListService` in `proto/tasks/v1/tasks.proto` replacing the current `TasksService` definition
2. THE Backend SHALL regenerate Connect_Handler code so that the generated Go package exposes `TaskListServiceHandler`, `UnimplementedTaskListServiceHandler`, and `NewTaskListServiceHandler` symbols
3. THE Backend SHALL update the Go server struct to embed `UnimplementedTaskListServiceHandler` instead of `UnimplementedTasksServiceHandler`
4. THE Backend SHALL update main.go to call `NewTaskListServiceHandler` and register the Reflector with the service name `tasks.v1.TaskListService`

### Requirement 3: Embed Note message in NoteService responses

**User Story:** As a developer, I want CreateNoteResponse and GetNoteResponse to embed the Note message instead of duplicating its fields, so that response payloads are DRY and consistent.

#### Acceptance Criteria

1. THE Backend SHALL define `CreateNoteResponse` with a single `Note note = 1` field instead of individual `file_path`, `title`, `content`, and `updated_at` fields
2. THE Backend SHALL define `GetNoteResponse` with a single `Note note = 1` field instead of individual `file_path`, `title`, `content`, and `updated_at` fields
3. THE Backend SHALL define `UpdateNoteResponse` with a single `Note note = 1` field replacing the current `updated_at`-only payload
4. THE Backend SHALL update the CreateNote handler to populate and return a `Note` message inside `CreateNoteResponse`
5. THE Backend SHALL update the GetNote handler to populate and return a `Note` message inside `GetNoteResponse`
6. THE Backend SHALL update the UpdateNote handler to read back the full note after writing and return a `Note` message inside `UpdateNoteResponse`

### Requirement 4: Unify mutation responses to return the affected resource

**User Story:** As a developer, I want all create and update RPCs to return the affected resource, so that clients always receive the current state of the resource after a mutation without a follow-up read.

#### Acceptance Criteria

1. THE NoteService CreateNote RPC SHALL return a `CreateNoteResponse` containing the full `Note` Affected_Resource
2. THE NoteService UpdateNote RPC SHALL return an `UpdateNoteResponse` containing the full `Note` Affected_Resource
3. THE FolderService CreateFolder RPC SHALL return a `CreateFolderResponse` containing the created `Folder` Affected_Resource instead of a sibling listing
4. THE FolderService UpdateFolder RPC SHALL return an `UpdateFolderResponse` containing the renamed `Folder` Affected_Resource instead of a sibling listing
5. THE TaskListService CreateTaskList RPC SHALL return a `CreateTaskListResponse` containing the full `TaskList` Affected_Resource
6. THE TaskListService UpdateTaskList RPC SHALL return an `UpdateTaskListResponse` containing the full `TaskList` Affected_Resource

### Requirement 5: Unify delete responses to return an empty response

**User Story:** As a developer, I want all delete RPCs to return only an empty success response or an error, so that delete semantics are uniform across all services.

#### Acceptance Criteria

1. THE NoteService DeleteNote RPC SHALL return an Empty_Response on success
2. THE FolderService DeleteFolder RPC SHALL return an Empty_Response on success instead of a sibling listing
3. THE TaskListService DeleteTaskList RPC SHALL return an Empty_Response on success
4. IF a delete target does not exist, THEN THE Backend SHALL return a `NotFound` error with a descriptive error message
5. IF a delete operation fails due to a filesystem error, THEN THE Backend SHALL return an `Internal` error with a descriptive error message

### Requirement 6: Add missing Folder CRUD RPCs

**User Story:** As a developer, I want FolderService to expose GetFolder, ListFolders, and UpdateFolder RPCs, so that the folder API has the same CRUD completeness as the other services.

#### Acceptance Criteria

1. THE FolderService SHALL expose a `GetFolder` RPC that accepts a folder path and returns the `Folder` Affected_Resource
2. THE FolderService SHALL expose a `ListFolders` RPC that accepts an optional parent path and returns a list of `Folder` entries that are immediate children of the specified parent
3. THE FolderService SHALL expose an `UpdateFolder` RPC that accepts a folder path and a new name, renames the folder, and returns the renamed `Folder` Affected_Resource
4. IF the requested folder path does not exist, THEN THE FolderService GetFolder RPC SHALL return a `NotFound` error with a descriptive error message
5. IF the requested parent path does not exist, THEN THE FolderService ListFolders RPC SHALL return a `NotFound` error with a descriptive error message
6. IF the target folder for UpdateFolder does not exist, THEN THE FolderService SHALL return a `NotFound` error with a descriptive error message
7. IF the new name in UpdateFolder conflicts with an existing sibling (case-insensitive), THEN THE FolderService SHALL return an `AlreadyExists` error with a descriptive error message

### Requirement 7: Define a Folder protobuf message

**User Story:** As a developer, I want a dedicated Folder message type in the protobuf schema, so that folder responses use a structured resource message consistent with Note and TaskList.

#### Acceptance Criteria

1. THE Backend SHALL define a `Folder` protobuf message in `proto/folder/v1/folder.proto` with `string path = 1` and `string name = 2` fields
2. THE FolderService CreateFolderResponse SHALL contain a single `Folder folder = 1` field
3. THE FolderService GetFolderResponse SHALL contain a single `Folder folder = 1` field
4. THE FolderService UpdateFolderResponse SHALL contain a single `Folder folder = 1` field
5. THE FolderService ListFoldersResponse SHALL contain a `repeated Folder folders = 1` field

### Requirement 8: Standardize RPC method naming across all services

**User Story:** As a developer, I want every service to follow the CreateXxx, GetXxx, ListXxxs, UpdateXxx, DeleteXxx naming convention, so that the API is predictable and self-documenting.

#### Acceptance Criteria

1. THE NoteService SHALL expose exactly these RPCs: `CreateNote`, `GetNote`, `ListNotes`, `UpdateNote`, `DeleteNote`
2. THE FolderService SHALL expose exactly these RPCs: `CreateFolder`, `GetFolder`, `ListFolders`, `UpdateFolder`, `DeleteFolder`
3. THE TaskListService SHALL expose exactly these RPCs: `CreateTaskList`, `GetTaskList`, `ListTaskLists`, `UpdateTaskList`, `DeleteTaskList`
4. THE FolderService SHALL remove the `RenameFolder` RPC, replacing its functionality with `UpdateFolder`

### Requirement 9: Update Go server wiring in main.go

**User Story:** As a developer, I want main.go to correctly register all renamed services and their handlers, so that the server compiles and serves the unified API.

#### Acceptance Criteria

1. THE Backend SHALL register NoteService using the generated `NewNoteServiceHandler` function in main.go
2. THE Backend SHALL register TaskListService using the generated `NewTaskListServiceHandler` function in main.go
3. THE Backend SHALL register FolderService using the generated `NewFolderServiceHandler` function in main.go
4. THE Backend SHALL configure the gRPC Reflector with service names `notes.v1.NoteService`, `folder.v1.FolderService`, and `tasks.v1.TaskListService`

### Requirement 10: Maintain existing TaskList response structure

**User Story:** As a developer, I want to confirm that TaskListService responses already embed the resource correctly, so that no unnecessary changes are made to the tasks proto.

#### Acceptance Criteria

1. THE TaskListService CreateTaskListResponse SHALL contain `file_path`, `name`, `tasks`, and `updated_at` fields matching the current definition
2. THE TaskListService GetTaskListResponse SHALL contain `file_path`, `name`, `tasks`, and `updated_at` fields matching the current definition
3. THE TaskListService UpdateTaskListResponse SHALL contain `file_path`, `name`, `tasks`, and `updated_at` fields matching the current definition
4. THE TaskListService DeleteTaskListResponse SHALL remain an Empty_Response
