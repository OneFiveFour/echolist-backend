# Design Document: API Unification

## Overview

This design standardizes the echolist backend's three domain services (NoteService, FolderService, TaskListService) to follow a uniform Connect/gRPC API convention. The changes are purely at the protobuf schema and Go handler layers — no new infrastructure, databases, or external dependencies are introduced.

Key changes:
- Rename `NotesService` → `NoteService` and `TasksService` → `TaskListService`
- Embed the `Note` message in all NoteService responses (eliminating field duplication)
- Define a `Folder` protobuf message and embed it in FolderService responses
- Add missing CRUD RPCs to FolderService (`GetFolder`, `ListFolders`, `UpdateFolder`)
- Replace `RenameFolder` with `UpdateFolder`
- Unify mutation responses to return the affected resource
- Unify delete responses to return an empty message
- Update `main.go` wiring and gRPC reflection

The TaskListService responses already follow the target pattern (inline resource fields), so they require only the service rename.

## Architecture

The system follows a straightforward layered architecture:

```
┌─────────────┐
│   Clients   │  (Connect/gRPC or HTTP/JSON)
└──────┬──────┘
       │
┌──────▼──────┐
│   main.go   │  HTTP mux + auth interceptor + reflection
└──────┬──────┘
       │
┌──────▼──────────────────────────────────────┐
│         Connect Service Handlers            │
│  ┌────────────┐ ┌──────────┐ ┌───────────┐ │
│  │NoteService │ │FolderSvc │ │TaskListSvc│ │
│  └─────┬──────┘ └────┬─────┘ └─────┬─────┘ │
│        │             │              │       │
│  server/        folder/        tasks/       │
└────────┼─────────────┼──────────────┼───────┘
         │             │              │
    ┌────▼─────────────▼──────────────▼────┐
    │          Filesystem (data/)          │
    └──────────────────────────────────────┘
```

All changes are confined to:
1. **Proto definitions** (`proto/*/v1/*.proto`) — schema changes
2. **Generated code** (`proto/gen/`) — regenerated via `buf generate`
3. **Handler packages** (`server/`, `folder/`, `tasks/`) — adapt to new generated types
4. **`main.go`** — update handler constructors and reflector names

No changes to `auth/`, `pathutil/`, or the filesystem layout.

## Components and Interfaces

### Proto Service Definitions (Target State)

**NoteService** (`proto/notes/v1/notes.proto`):
```protobuf
service NoteService {
  rpc CreateNote (CreateNoteRequest) returns (CreateNoteResponse);
  rpc GetNote (GetNoteRequest) returns (GetNoteResponse);
  rpc ListNotes (ListNotesRequest) returns (ListNotesResponse);
  rpc UpdateNote (UpdateNoteRequest) returns (UpdateNoteResponse);
  rpc DeleteNote (DeleteNoteRequest) returns (DeleteNoteResponse);
}
```

**FolderService** (`proto/folder/v1/folder.proto`):
```protobuf
service FolderService {
  rpc CreateFolder (CreateFolderRequest) returns (CreateFolderResponse);
  rpc GetFolder (GetFolderRequest) returns (GetFolderResponse);
  rpc ListFolders (ListFoldersRequest) returns (ListFoldersResponse);
  rpc UpdateFolder (UpdateFolderRequest) returns (UpdateFolderResponse);
  rpc DeleteFolder (DeleteFolderRequest) returns (DeleteFolderResponse);
}
```

**TaskListService** (`proto/tasks/v1/tasks.proto`):
```protobuf
service TaskListService {
  rpc CreateTaskList (CreateTaskListRequest) returns (CreateTaskListResponse);
  rpc GetTaskList (GetTaskListRequest) returns (GetTaskListResponse);
  rpc ListTaskLists (ListTaskListsRequest) returns (ListTaskListsResponse);
  rpc UpdateTaskList (UpdateTaskListRequest) returns (UpdateTaskListResponse);
  rpc DeleteTaskList (DeleteTaskListRequest) returns (DeleteTaskListResponse);
}
```

### Go Handler Changes

**`server/` package (NoteService)**:
- Rename `NotesServer` struct → embed `UnimplementedNoteServiceHandler`
- `CreateNote`: build `&pb.Note{...}` and return `&pb.CreateNoteResponse{Note: note}`
- `GetNote`: build `&pb.Note{...}` and return `&pb.GetNoteResponse{Note: note}`
- `UpdateNote`: after writing, read back the file to populate full `Note`, return `&pb.UpdateNoteResponse{Note: note}`
- `DeleteNote`: no response body change (already empty)
- `ListNotes`: no change (already uses `repeated Note`)

**`folder/` package (FolderService)**:
- Rename embedded handler to `UnimplementedFolderServiceHandler` (already correct name, but generated code changes)
- Remove `RenameFolder` method, replace with `UpdateFolder` using same logic
- `CreateFolder`: return `&pb.CreateFolderResponse{Folder: &pb.Folder{Path: ..., Name: ...}}` instead of entries listing
- `UpdateFolder`: return `&pb.UpdateFolderResponse{Folder: &pb.Folder{Path: ..., Name: ...}}`
- `DeleteFolder`: return `&pb.DeleteFolderResponse{}` (empty, no more entries listing)
- Add `GetFolder`: stat the folder, return `Folder` message
- Add `ListFolders`: read directory children, return `repeated Folder`
- Remove `listDirectory` helper (no longer needed for responses) or repurpose for `ListFolders`

**`tasks/` package (TaskListService)**:
- Rename `TaskServer` struct → embed `UnimplementedTaskListServiceHandler`
- Response payloads stay the same (already inline resource fields)

**`main.go`**:
- `notesv1connect.NewNoteServiceHandler(...)` instead of `NewNotesServiceHandler`
- `tasksv1connect.NewTaskListServiceHandler(...)` instead of `NewTasksServiceHandler`
- Reflector names: `"notes.v1.NoteService"`, `"tasks.v1.TaskListService"`, `"folder.v1.FolderService"`


## Data Models

### Protobuf Messages (Target State)

**Note** (unchanged):
```protobuf
message Note {
  string file_path = 1;
  string title = 2;
  string content = 3;
  int64 updated_at = 4;
}
```

**Folder** (new):
```protobuf
message Folder {
  string path = 1;
  string name = 2;
}
```

**TaskList-related messages** (unchanged — `Subtask`, `MainTask`, `TaskListEntry` remain as-is).

### NoteService Messages (Changed)

```protobuf
// Before: had file_path, title, content, updated_at as top-level fields
// After: embeds Note
message CreateNoteResponse { Note note = 1; }
message GetNoteResponse    { Note note = 1; }
message UpdateNoteResponse { Note note = 1; }  // was: only updated_at
message DeleteNoteResponse {}                   // unchanged
```

### FolderService Messages (Changed)

```protobuf
message Folder { string path = 1; string name = 2; }

message CreateFolderRequest  { string parent_path = 1; string name = 2; }
message CreateFolderResponse { Folder folder = 1; }  // was: repeated FolderEntry

message GetFolderRequest  { string folder_path = 1; }
message GetFolderResponse { Folder folder = 1; }     // new

message ListFoldersRequest  { string parent_path = 1; }
message ListFoldersResponse { repeated Folder folders = 1; }  // new

message UpdateFolderRequest  { string folder_path = 1; string new_name = 2; }
message UpdateFolderResponse { Folder folder = 1; }  // replaces RenameFolder

message DeleteFolderRequest  { string folder_path = 1; }
message DeleteFolderResponse {}  // was: repeated FolderEntry
```

The `FolderEntry` and `RenameFolderRequest`/`RenameFolderResponse` messages are removed.

### TaskListService Messages (Unchanged)

All request/response messages remain identical. Only the service name changes from `TasksService` to `TaskListService`.


## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Note create-then-get round trip

*For any* valid note title and content, creating a note via `CreateNote` and then retrieving it via `GetNote` using the returned `file_path` should produce a `Note` with the same `title`, `content`, and `file_path`. The `updated_at` should be non-zero in both responses.

**Validates: Requirements 3.1, 3.2, 3.4, 3.5, 4.1**

### Property 2: UpdateNote returns full Note with updated content

*For any* existing note and any new content string, calling `UpdateNote` should return an `UpdateNoteResponse` containing a `Note` whose `content` matches the new content, whose `file_path` and `title` match the original note, and whose `updated_at` is non-zero.

**Validates: Requirements 3.3, 3.6, 4.2**

### Property 3: CreateFolder returns correct Folder

*For any* valid folder name and existing parent path, calling `CreateFolder` should return a `CreateFolderResponse` containing a `Folder` whose `name` matches the requested name and whose `path` is the concatenation of the parent path and the name (with trailing `/`).

**Validates: Requirements 4.3, 7.2**

### Property 4: GetFolder returns correct Folder

*For any* existing folder, calling `GetFolder` with its path should return a `GetFolderResponse` containing a `Folder` whose `path` matches the request path and whose `name` matches the folder's directory name.

**Validates: Requirements 6.1, 7.3**

### Property 5: UpdateFolder returns renamed Folder

*For any* existing folder and any valid new name that does not conflict with siblings, calling `UpdateFolder` should return an `UpdateFolderResponse` containing a `Folder` whose `name` matches the new name and whose `path` reflects the renamed location.

**Validates: Requirements 4.4, 6.3, 7.4**

### Property 6: ListFolders returns immediate children

*For any* parent directory containing a known set of child folders, calling `ListFolders` should return a `ListFoldersResponse` whose `folders` list contains exactly one `Folder` entry per immediate child directory, each with the correct `path` and `name`.

**Validates: Requirements 6.2, 7.5**

### Property 7: Non-existent folder path returns NotFound

*For any* folder path that does not exist on the filesystem, calling `GetFolder`, `ListFolders`, `UpdateFolder`, or `DeleteFolder` with that path should return a Connect error with code `NotFound`.

**Validates: Requirements 5.4, 6.4, 6.5, 6.6**

### Property 8: UpdateFolder case-insensitive sibling conflict returns AlreadyExists

*For any* existing folder and any new name that case-insensitively matches an existing sibling folder's name, calling `UpdateFolder` should return a Connect error with code `AlreadyExists`.

**Validates: Requirements 6.7**

## Error Handling

All services follow a consistent error handling pattern using Connect error codes:

| Condition | Error Code | Example |
|---|---|---|
| Required field empty or invalid | `InvalidArgument` | Empty folder name, path with separators |
| Path traversal outside data dir | `InvalidArgument` | `../../etc/passwd` |
| Resource not found | `NotFound` | GetFolder on non-existent path |
| Name conflict (case-insensitive) | `AlreadyExists` | CreateFolder/UpdateFolder with duplicate name |
| Filesystem I/O failure | `Internal` | Disk full, permission denied |

Specific error handling changes in this spec:

- **DeleteFolder**: Currently returns entries listing on success. Will return empty response. On NotFound, returns `NotFound` error (existing behavior preserved).
- **DeleteNote**: Already returns empty response and raw `os` errors. Should be updated to return proper Connect `NotFound` and `Internal` error codes.
- **New RPCs (GetFolder, ListFolders, UpdateFolder)**: Follow the same pattern as existing RPCs — validate input, check existence, perform operation, return result or Connect error.

## Testing Strategy

### Property-Based Testing

Use the `pgregory.net/rapid` library (already used in the project for `server/createNote_property_test.go` and `tasks/parser_property_test.go`).

Each property test runs a minimum of 100 iterations with randomly generated inputs. Each test is tagged with a comment referencing the design property:

```go
// Feature: api-unification, Property 1: Note create-then-get round trip
```

Property tests to implement:
1. **Note round trip** (Property 1): Generate random titles/content, create note, get note, assert equality
2. **UpdateNote full response** (Property 2): Create note, update with random content, assert response Note fields
3. **CreateFolder response** (Property 3): Generate random valid folder names, create, assert Folder fields
4. **GetFolder response** (Property 4): Create folder, get it, assert Folder fields
5. **UpdateFolder response** (Property 5): Create folder, rename with random valid name, assert Folder fields
6. **ListFolders children** (Property 6): Create random number of child folders, list, assert exact match
7. **NotFound on missing path** (Property 7): Generate random non-existent paths, call Get/List/Update/Delete, assert NotFound
8. **Case-insensitive conflict** (Property 8): Create two folders, attempt rename to case-variant of sibling, assert AlreadyExists

### Unit Testing

Unit tests complement property tests for specific examples and edge cases:

- **Delete empty response**: Verify DeleteFolder returns empty response (not entries listing)
- **UpdateNote reads back content**: Verify the handler reads the file after writing (not just echoing input)
- **Reflection names**: Verify main.go registers correct service names (integration-level)
- **Edge cases**: Empty paths, root-level operations, special characters in names

### Test Organization

- `server/*_test.go` — NoteService handler tests
- `folder/*_test.go` — FolderService handler tests (new files for GetFolder, ListFolders, UpdateFolder)
- `tasks/*_test.go` — TaskListService handler tests (minimal changes, mostly rename verification)
- Property tests use `_property_test.go` suffix convention (already established in the project)
