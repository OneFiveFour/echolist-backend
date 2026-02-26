# Implementation Plan: API Unification

## Overview

Standardize the echolist backend's three domain services to follow a uniform Connect/gRPC API convention. Proto schema changes come first (source of truth), followed by code generation, handler updates, main.go wiring, and tests. All code is Go, using `pgregory.net/rapid` for property-based tests.

## Tasks

- [x] 1. Update proto definitions for NoteService
  - [x] 1.1 Rename `NotesService` to `NoteService` and embed `Note` in responses
    - In `proto/notes/v1/notes.proto`: rename `service NotesService` to `service NoteService`
    - Replace `CreateNoteResponse` fields (`file_path`, `title`, `content`, `updated_at`) with a single `Note note = 1` field
    - Replace `GetNoteResponse` fields (`file_path`, `title`, `content`, `updated_at`) with a single `Note note = 1` field
    - Replace `UpdateNoteResponse` field (`updated_at`) with a single `Note note = 1` field
    - `DeleteNoteResponse` and `ListNotesResponse` remain unchanged
    - _Requirements: 1.1, 3.1, 3.2, 3.3, 8.1_

- [ ] 2. Update proto definitions for FolderService
  - [ ] 2.1 Define `Folder` message and add missing CRUD RPCs
    - In `proto/folder/v1/folder.proto`: add `message Folder { string path = 1; string name = 2; }`
    - Remove `FolderEntry`, `RenameFolderRequest`, `RenameFolderResponse` messages
    - Replace `RenameFolder` RPC with `UpdateFolder(UpdateFolderRequest) returns (UpdateFolderResponse)`
    - Add `GetFolder(GetFolderRequest) returns (GetFolderResponse)` RPC
    - Add `ListFolders(ListFoldersRequest) returns (ListFoldersResponse)` RPC
    - Define new request/response messages: `GetFolderRequest { string folder_path = 1; }`, `GetFolderResponse { Folder folder = 1; }`, `ListFoldersRequest { string parent_path = 1; }`, `ListFoldersResponse { repeated Folder folders = 1; }`, `UpdateFolderRequest { string folder_path = 1; string new_name = 2; }`, `UpdateFolderResponse { Folder folder = 1; }`
    - Update `CreateFolderResponse` to contain `Folder folder = 1` instead of `repeated FolderEntry entries`
    - Update `DeleteFolderResponse` to be empty (remove `repeated FolderEntry entries`)
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 8.2, 8.4_

- [ ] 3. Update proto definition for TaskListService
  - [ ] 3.1 Rename `TasksService` to `TaskListService`
    - In `proto/tasks/v1/tasks.proto`: rename `service TasksService` to `service TaskListService`
    - All request/response messages remain unchanged
    - _Requirements: 2.1, 8.3, 10.1, 10.2, 10.3, 10.4_

- [ ] 4. Regenerate Connect/gRPC code
  - [ ] 4.1 Run `buf generate` to regenerate all Go code in `proto/gen/`
    - Run `buf generate` from the project root
    - Verify that `proto/gen/notes/v1/notesv1connect/` exposes `NoteServiceHandler`, `UnimplementedNoteServiceHandler`, `NewNoteServiceHandler`
    - Verify that `proto/gen/folder/v1/folderv1connect/` exposes `FolderServiceHandler`, `UnimplementedFolderServiceHandler`, `NewFolderServiceHandler`
    - Verify that `proto/gen/tasks/v1/tasksv1connect/` exposes `TaskListServiceHandler`, `UnimplementedTaskListServiceHandler`, `NewTaskListServiceHandler`
    - _Requirements: 1.2, 2.2_

- [ ] 5. Checkpoint - Verify proto generation
  - Ensure `buf generate` completes without errors, ask the user if questions arise.

- [ ] 6. Update NoteService handlers
  - [ ] 6.1 Update `server/notesServer.go` struct embedding
    - Change `NotesServer` struct to embed `notesv1connect.UnimplementedNoteServiceHandler` instead of `UnimplementedNotesServiceHandler`
    - _Requirements: 1.3_
  - [ ] 6.2 Update `CreateNote` handler to return embedded `Note`
    - In `server/createNote.go`: build a `&pb.Note{...}` and return `&pb.CreateNoteResponse{Note: note}`
    - _Requirements: 3.4, 4.1_
  - [ ] 6.3 Update `GetNote` handler to return embedded `Note`
    - In `server/getNote.go`: build a `&pb.Note{...}` and return `&pb.GetNoteResponse{Note: note}`
    - _Requirements: 3.5_
  - [ ] 6.4 Update `UpdateNote` handler to return full `Note`
    - In `server/updateNote.go`: after writing, read back the file to populate full `Note` fields, return `&pb.UpdateNoteResponse{Note: note}`
    - _Requirements: 3.3, 3.6, 4.2_
  - [ ]* 6.5 Write property test: Note create-then-get round trip
    - **Property 1: Note create-then-get round trip**
    - Create a note with random title/content via `CreateNote`, retrieve via `GetNote`, assert `title`, `content`, `file_path` match and `updated_at` is non-zero in both responses
    - **Validates: Requirements 3.1, 3.2, 3.4, 3.5, 4.1**
  - [ ]* 6.6 Write property test: UpdateNote returns full Note
    - **Property 2: UpdateNote returns full Note with updated content**
    - Create a note, update with random content, assert response `Note` has matching `content`, original `file_path`/`title`, and non-zero `updated_at`
    - **Validates: Requirements 3.3, 3.6, 4.2**

- [ ] 7. Update FolderService handlers
  - [ ] 7.1 Update `folder/folder_server.go` struct and helpers
    - Update the `FolderServer` struct embedding if the generated handler name changed
    - Remove or repurpose the `listDirectory` helper (no longer returns `FolderEntry` slices)
    - _Requirements: 8.2_
  - [ ] 7.2 Update `CreateFolder` handler to return `Folder` message
    - In `folder/create_folder.go`: return `&pb.CreateFolderResponse{Folder: &pb.Folder{Path: ..., Name: ...}}` instead of entries listing
    - _Requirements: 4.3, 7.2_
  - [ ] 7.3 Implement `GetFolder` handler
    - Create `folder/get_folder.go`: stat the folder path, validate it exists and is a directory, return `&pb.GetFolderResponse{Folder: &pb.Folder{Path: ..., Name: ...}}`
    - Return `NotFound` error if path does not exist
    - _Requirements: 6.1, 6.4, 7.3_
  - [ ] 7.4 Implement `ListFolders` handler
    - Create `folder/list_folders.go`: read directory children, filter to directories only, return `&pb.ListFoldersResponse{Folders: []*pb.Folder{...}}`
    - Return `NotFound` error if parent path does not exist
    - _Requirements: 6.2, 6.5, 7.5_
  - [ ] 7.5 Replace `RenameFolder` with `UpdateFolder` handler
    - Rename `folder/rename_folder.go` to `folder/update_folder.go`
    - Change method signature from `RenameFolder` to `UpdateFolder` using new request/response types
    - Return `&pb.UpdateFolderResponse{Folder: &pb.Folder{Path: ..., Name: ...}}` instead of entries listing
    - Add case-insensitive sibling conflict check returning `AlreadyExists` error
    - _Requirements: 4.4, 6.3, 6.6, 6.7, 7.4, 8.4_
  - [ ] 7.6 Update `DeleteFolder` handler to return empty response
    - In `folder/delete_folder.go`: return `&pb.DeleteFolderResponse{}` (empty) instead of entries listing
    - Return `NotFound` error if path does not exist, `Internal` error on filesystem failure
    - _Requirements: 5.2, 5.4, 5.5_
  - [ ]* 7.7 Write property test: CreateFolder returns correct Folder
    - **Property 3: CreateFolder returns correct Folder**
    - Generate random valid folder names, create folder, assert `Folder.name` matches request and `Folder.path` is parent + name with trailing `/`
    - **Validates: Requirements 4.3, 7.2**
  - [ ]* 7.8 Write property test: GetFolder returns correct Folder
    - **Property 4: GetFolder returns correct Folder**
    - Create a folder, call `GetFolder`, assert `Folder.path` and `Folder.name` match
    - **Validates: Requirements 6.1, 7.3**
  - [ ]* 7.9 Write property test: UpdateFolder returns renamed Folder
    - **Property 5: UpdateFolder returns renamed Folder**
    - Create folder, rename with random valid name, assert response `Folder.name` matches new name and `Folder.path` reflects renamed location
    - **Validates: Requirements 4.4, 6.3, 7.4**
  - [ ]* 7.10 Write property test: ListFolders returns immediate children
    - **Property 6: ListFolders returns immediate children**
    - Create random number of child folders under a parent, call `ListFolders`, assert exact match of child `Folder` entries
    - **Validates: Requirements 6.2, 7.5**
  - [ ]* 7.11 Write property test: Non-existent folder path returns NotFound
    - **Property 7: Non-existent folder path returns NotFound**
    - Generate random non-existent paths, call `GetFolder`, `ListFolders`, `UpdateFolder`, `DeleteFolder`, assert `NotFound` error code
    - **Validates: Requirements 5.4, 6.4, 6.5, 6.6**
  - [ ]* 7.12 Write property test: Case-insensitive sibling conflict returns AlreadyExists
    - **Property 8: UpdateFolder case-insensitive sibling conflict returns AlreadyExists**
    - Create two sibling folders, attempt rename of one to case-variant of the other's name, assert `AlreadyExists` error
    - **Validates: Requirements 6.7**

- [ ] 8. Update TaskListService handlers
  - [ ] 8.1 Update `tasks/task_server.go` struct embedding
    - Change `TaskServer` struct to embed `tasksv1connect.UnimplementedTaskListServiceHandler` instead of `UnimplementedTasksServiceHandler`
    - _Requirements: 2.3_

- [ ] 9. Checkpoint - Verify handler compilation
  - Ensure all handler packages compile without errors, ask the user if questions arise.

- [ ] 10. Update main.go wiring
  - [ ] 10.1 Update service handler registration and reflector names
    - Replace `notesv1connect.NewNotesServiceHandler(...)` with `notesv1connect.NewNoteServiceHandler(...)`
    - Replace `tasksv1connect.NewTasksServiceHandler(...)` with `tasksv1connect.NewTaskListServiceHandler(...)`
    - Update reflector service names to `"notes.v1.NoteService"`, `"folder.v1.FolderService"`, `"tasks.v1.TaskListService"`
    - _Requirements: 1.4, 2.4, 9.1, 9.2, 9.3, 9.4_

- [ ] 11. Update existing tests
  - [ ] 11.1 Update NoteService test files for new response types
    - Update `server/createNote_test.go`, `server/getNote_test.go`, `server/deleteNote_test.go` to use new response types (e.g., `resp.Note.FilePath` instead of `resp.FilePath`)
    - Update `server/createNote_property_test.go` and `server/listNotes_property_test.go` for any type changes
    - _Requirements: 3.1, 3.2, 3.4, 3.5_
  - [ ] 11.2 Update FolderService test files for new types and method names
    - Update `folder/create_folder_test.go` to assert on `Folder` message instead of `FolderEntry` list
    - Update `folder/error_conditions_test.go` to replace `RenameFolder` calls with `UpdateFolder`
    - Update `folder/rename_delete_test.go` to use `UpdateFolder` and new response types
    - _Requirements: 7.2, 7.4, 8.4_
  - [ ] 11.3 Update TaskListService test files for renamed service
    - Update `tasks/task_server_property_test.go` and other task test files if they reference `TasksService` types
    - _Requirements: 2.3_

- [ ] 12. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Proto changes (tasks 1-3) must be completed before code generation (task 4)
- Code generation (task 4) must complete before handler updates (tasks 6-8)
- Handler updates must complete before main.go wiring (task 10)
- Property tests use `pgregory.net/rapid` (already in the project)
- The `updateNote.go` handler needs a read-back after write to populate the full `Note` response
- TaskListService responses are already correct; only the service rename is needed
