# Implementation Plan: File Service Refactor

## Overview

Rename the existing `FolderService` Connect-RPC service to `FileService`, remove the `GetFolder` RPC, implement the new `ListFiles` RPC, and migrate all existing folder RPCs to the new `file/` package. The migration is incremental: proto first, then generated code, then Go implementation, then wiring, then cleanup.

## Tasks

- [x] 1. Create the new proto definition and generate Go code
  - [x] 1.1 Create `proto/file/v1/file.proto` with the `FileService` definition
    - Define `file.v1` package with `FileService` exposing `CreateFolder`, `ListFiles`, `UpdateFolder`, `DeleteFolder`
    - Include all request/response messages (`Folder`, `CreateFolderRequest`, `CreateFolderResponse`, `ListFilesRequest`, `ListFilesResponse`, `UpdateFolderRequest`, `UpdateFolderResponse`, `DeleteFolderRequest`, `DeleteFolderResponse`)
    - `GetFolder` RPC and its messages (`GetFolderRequest`, `GetFolderResponse`) must NOT be included
    - `ListFiles` returns `repeated string entries` instead of `repeated Folder folders`
    - Set `option go_package = "gen/file;file";`
    - _Requirements: 1.1, 1.2, 2.1, 3.1, 3.2_
  - [x] 1.2 Run `buf generate` to produce Go code under `proto/gen/file/v1/` and `proto/gen/file/v1/filev1connect/`
    - Verify the generated `filev1connect` package contains `NewFileServiceHandler` and `UnimplementedFileServiceHandler`
    - _Requirements: 1.3_

- [x] 2. Implement the `file/` Go package
  - [x] 2.1 Create `file/file_server.go` with `FileServer` struct and constructor
    - Define `FileServer` struct embedding `filev1connect.UnimplementedFileServiceHandler` and holding `dataDir`
    - Implement `NewFileServer(dataDir string) *FileServer` constructor
    - Migrate `validateName` helper from `folder/folder_server.go` (no logic changes)
    - _Requirements: 1.4, 4.1_
  - [x] 2.2 Migrate `CreateFolder` RPC to `file/create_folder.go`
    - Copy from `folder/create_folder.go`, update package name to `file`, update imports from `folderv1`/`folderv1connect` to `filev1`/`filev1connect`
    - Change receiver from `FolderServer` to `FileServer`
    - No logic changes
    - _Requirements: 4.1, 4.2_
  - [x] 2.3 Migrate `UpdateFolder` RPC to `file/update_folder.go`
    - Copy from `folder/update_folder.go`, update package name and imports
    - Change receiver from `FolderServer` to `FileServer`
    - No logic changes
    - _Requirements: 4.1, 4.2_
  - [x] 2.4 Migrate `DeleteFolder` RPC to `file/delete_folder.go`
    - Copy from `folder/delete_folder.go`, update package name and imports
    - Change receiver from `FolderServer` to `FileServer`
    - No logic changes
    - _Requirements: 4.1, 4.2_
  - [x] 2.5 Implement `ListFiles` RPC in `file/list_files.go`
    - Base on `folder/list_folders.go` but return `[]string` entries instead of `[]*Folder`
    - Iterate all entries from `os.ReadDir` (not just directories)
    - Append `/` suffix to directory entries, no suffix for file entries
    - Validate parent path with `pathutil.IsSubPath`; return `InvalidArgument` if path escapes data directory
    - Return `NotFound` if parent path does not exist
    - Return `NotFound` with "not a directory" message if parent path is a file
    - Empty/root parent path resolves to `dataDir`
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

- [x] 3. Checkpoint - Verify compilation
  - Ensure the `file/` package compiles with no errors, ask the user if questions arise.

- [x] 4. Migrate and write tests for the `file/` package
  - [x] 4.1 Migrate existing property tests to `file/` package
    - Copy `folder/create_folder_test.go` → `file/create_folder_test.go`, update package to `file`, imports to `filev1`
    - Copy `folder/rename_delete_test.go` → `file/rename_delete_test.go`, update package and imports
    - Copy `folder/folder_api_property_test.go` → `file/file_api_property_test.go`, update package and imports
    - Remove `TestProperty4_GetFolderReturnsCorrectFolder` (GetFolder is removed)
    - Update `TestProperty7_NonExistentFolderReturnsNotFound` to remove the `GetFolder` assertion
    - Change all `NewFolderServer` calls to `NewFileServer`, `FolderServer` to `FileServer`
    - _Requirements: 2.1, 2.2, 4.1, 4.2_
  - [x] 4.2 Migrate existing error condition unit tests to `file/` package
    - Copy `folder/error_conditions_test.go` → `file/error_conditions_test.go`, update package and imports
    - Change all `NewFolderServer` calls to `NewFileServer`
    - _Requirements: 4.1, 4.2_
  - [x]* 4.3 Write property test for ListFiles immediate children with correct entry format
    - **Property 1: ListFiles returns immediate children with correct entry format**
    - Generate random directory trees with files and subdirs, call `ListFiles`, verify: (a) every immediate child represented exactly once, (b) directory entries end with `/`, (c) file entries do not end with `/`, (d) no deeper entries included
    - Place in `file/list_files_test.go`
    - **Validates: Requirements 3.1, 3.2, 3.3, 3.4**
  - [x]* 4.4 Write property test for ListFiles non-existent path
    - **Property 2: ListFiles on non-existent path returns NotFound**
    - Generate random non-existent path strings, verify `NotFound` error
    - Place in `file/list_files_test.go`
    - **Validates: Requirements 3.5**
  - [x]* 4.5 Write property test for ListFiles on file path
    - **Property 3: ListFiles on file path returns NotFound**
    - Create random files, call `ListFiles` with file paths, verify `NotFound` error with "not a directory" message
    - Place in `file/list_files_test.go`
    - **Validates: Requirements 3.6**
  - [x]* 4.6 Write property test for ListFiles path traversal
    - **Property 4: ListFiles on path-traversal returns InvalidArgument**
    - Generate path-traversal strings (e.g. `../` sequences), verify `InvalidArgument` error
    - Place in `file/list_files_test.go`
    - **Validates: Requirements 3.7**
  - [x]* 4.7 Write unit tests for ListFiles edge cases
    - `TestListFiles_EmptyDirectory` — empty directory returns empty entries list
    - `TestListFiles_RootPath` — empty `parent_path` returns data directory children
    - Place in `file/list_files_test.go`
    - _Requirements: 3.1, 3.4_

- [x] 5. Checkpoint - Run all tests in `file/` package
  - Ensure all tests pass, ask the user if questions arise.

- [x] 6. Update `main.go` wiring and clean up old packages
  - [x] 6.1 Update `main.go` to use `FileService`
    - Replace `folder` import with `file` import (`echolist-backend/file`)
    - Replace `folderv1connect` import with `filev1connect` import (`echolist-backend/proto/gen/file/v1/filev1connect`)
    - Replace `folderv1connect.NewFolderServiceHandler(folder.NewFolderServer(dataDir), interceptors)` with `filev1connect.NewFileServiceHandler(file.NewFileServer(dataDir), interceptors)`
    - Update gRPC reflection service name from `"folder.v1.FolderService"` to `"file.v1.FileService"`
    - _Requirements: 1.5, 1.6_
  - [x] 6.2 Remove old `folder/`, `proto/folder/`, and `proto/gen/folder/` directories
    - Delete `folder/` Go package directory
    - Delete `proto/folder/` proto source directory
    - Delete `proto/gen/folder/` generated code directory
    - _Requirements: 1.6_

- [x] 7. Final checkpoint - Full build and test verification
  - Ensure the project compiles and all tests pass across all packages, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- The design uses Go, so all implementation tasks use Go
- The migrated RPCs (CreateFolder, UpdateFolder, DeleteFolder) require import updates only — no logic changes
- The only new logic is in `ListFiles` (task 2.5)
- Property tests use `pgregory.net/rapid` (already a project dependency)
- NoteService and TaskListService are not modified (Requirement 5)
