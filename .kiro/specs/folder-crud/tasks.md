# Implementation Plan: Folder CRUD

## Overview

Implement a standalone FolderService with Create, Rename, and Delete RPCs, plus update ListNotes to return shallow unified directory listings. The implementation follows the existing project patterns (Connect/gRPC, filesystem-backed, `pgregory.net/rapid` for property tests).

## Tasks

- [-] 1. Define proto and generate code
  - [ ] 1.1 Create `proto/folder/v1/folder.proto` with FolderService definition (CreateFolder, RenameFolder, DeleteFolder RPCs and all request/response messages including DirectoryEntry)
    - _Requirements: 1.1, 2.1, 3.1_
  - [ ] 1.2 Update `proto/notes/v1/notes.proto` to add `repeated string entries` field to `ListNotesResponse`
    - _Requirements: 4.1, 4.2_
  - [ ] 1.3 Run `buf generate` to regenerate Go code from proto definitions
    - _Requirements: 1.1, 2.1, 3.1, 4.1_

- [ ] 2. Implement FolderServer core and helpers
  - [ ] 2.1 Create `folder/folder_server.go` with FolderServer struct, constructor, path validation helper (`isSubPath`), name validation helper, and `listDirectory` helper that reads immediate children and returns DirectoryEntry slices
    - _Requirements: 1.1, 1.3, 1.4, 1.5_
  - [ ] 2.2 Implement `CreateFolder` in `folder/create_folder.go` — validate name, resolve paths, check case-insensitive duplicates via `os.ReadDir` + `strings.EqualFold`, create directory with `os.Mkdir`, return parent listing
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5_
  - [ ] 2.3 Write property test for CreateFolder round-trip
    - **Property 1: Create folder round-trip**
    - **Validates: Requirements 1.1, 1.4**
  - [ ] 2.4 Write property test for case-insensitive duplicate rejection on create
    - **Property 2: Case-insensitive duplicate rejection on create**
    - **Validates: Requirements 1.2**
  - [ ] 2.5 Write property test for invalid name rejection
    - **Property 3: Invalid name rejection**
    - **Validates: Requirements 1.3, 2.4**

- [x] 3. Implement RenameFolder and DeleteFolder
  - [x] 3.1 Implement `RenameFolder` in `folder/rename_folder.go` — validate new name, check folder exists, check case-insensitive sibling duplicates, rename with `os.Rename`, return parent listing
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_
  - [x] 3.2 Implement `DeleteFolder` in `folder/delete_folder.go` — validate non-empty path, check folder exists, remove with `os.RemoveAll`, return parent listing
    - _Requirements: 3.1, 3.2, 3.3, 3.4_
  - [x] 3.3 Write property test for rename preserves contents
    - **Property 4: Rename preserves contents**
    - **Validates: Requirements 2.1, 2.5**
  - [x] 3.4 Write property test for case-insensitive duplicate rejection on rename
    - **Property 5: Case-insensitive duplicate rejection on rename**
    - **Validates: Requirements 2.2**
  - [x] 3.5 Write property test for delete removes folder and contents
    - **Property 6: Delete removes folder and contents**
    - **Validates: Requirements 3.1, 3.4**
  - [x] 3.6 Write unit tests for error conditions (non-existent folder, deleting root, path traversal)
    - _Requirements: 1.5, 2.3, 3.2, 3.3_

- [x] 4. Checkpoint - Ensure all FolderService tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 5. Update ListNotes to shallow listing with folders
  - [x] 5.1 Refactor `server/listNotes.go` — replace `filepath.Walk` with `os.ReadDir` for shallow listing, populate both `notes` (Note objects for .md files) and `entries` (unified path strings with trailing `/` for folders)
    - _Requirements: 4.1, 4.2, 4.3_
  - [x] 5.2 Update `server/listNotes_test.go` to reflect new shallow listing behavior and verify folder entries
    - _Requirements: 4.1, 4.2, 4.3_
  - [x] 5.3 Write property test for ListNotes returns all immediate children with correct formatting
    - **Property 7: ListNotes returns all immediate children with correct formatting**
    - **Validates: Requirements 4.1, 4.2**
  - [x] 5.4 Write property test for ListNotes shallow listing
    - **Property 8: ListNotes shallow listing**
    - **Validates: Requirements 4.3**

- [ ] 6. Wire FolderService into main.go
  - [ ] 6.1 Register FolderService handler in `main.go` — import generated connect package, create FolderServer with dataDir, register on mux with auth interceptor, add to gRPC reflector
    - _Requirements: 1.1, 2.1, 3.1_

- [ ] 7. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Property tests use `pgregory.net/rapid` (already in go.mod)
- Proto code generation requires `buf generate` from the `proto/` directory
- The FolderServer lives in a new `folder/` package at the project root
