# Implementation Plan: Code Review Hardening

## Overview

Harden the echolist-backend against 18 code review findings: security fixes (path traversal, panic-safe title extraction, existence checks, duplicate detection, null byte injection, token type confusion), API consistency (proto field rename, error messages, log language), code deduplication (shared title extraction, path validation consolidation, shared task validation), and operational improvements (no-op removal, graceful shutdown, request size limits, fsync, username enumeration prevention). All changes are localized refactors — no new services or external dependencies.

## Tasks

- [x] 1. Proto changes and code regeneration
  - [x] 1.1 Rename `path` to `parent_dir` in `CreateNoteRequest` and `ListNotesRequest` in `proto/notes/v1/notes.proto`, rename `parent_path` to `parent_dir` in `CreateFolderRequest` and `ListFilesRequest` in `proto/file/v1/file.proto`, and rename `path` to `parent_dir` in `CreateTaskListRequest` and `ListTaskListsRequest` in `proto/tasks/v1/tasks.proto`
    - Change field names keeping field numbers unchanged
    - _Requirements: 8.1, 8.3_
  - [x] 1.2 Run `buf generate` to regenerate Go code from updated proto definitions
    - _Requirements: 8.2_
  - [x] 1.3 Update `server/createNote.go`, `server/listNotes.go`, `file/create_folder.go`, `file/list_files.go`, `tasks/create_task_list.go`, and `tasks/list_task_lists.go` to use `req.GetParentDir()` instead of `req.GetPath()` or `req.GetParentPath()`
    - Update all handler code and any test files referencing the old field names
    - _Requirements: 8.2_

- [x] 2. Shared title extraction and panic safety
  - [x] 2.1 Create `server/title.go` with `ExtractNoteTitle(filename string) (string, error)` function
    - Handle filenames shorter than expected prefix+suffix length by returning an error
    - Validate `note_` prefix and `.md` suffix
    - _Requirements: 2.1, 2.2, 2.3, 11.1_
  - [x] 2.2 Replace inline title extraction in `server/getNote.go`, `server/updateNote.go`, and `server/listNotes.go` with calls to `ExtractNoteTitle`
    - In GetNote and UpdateNote, return `connect.NewError(connect.CodeInternal, ...)` when extraction fails
    - _Requirements: 2.1, 2.2, 11.2, 11.3_
  - [x] 2.3 Write property test `TestProperty_ExtractNoteTitleNeverPanics` in `server/title_test.go`
    - **Property 2: Title extraction never panics**
    - Generate random strings of all lengths (including empty), call `ExtractNoteTitle`, verify no panic
    - **Validates: Requirements 2.1, 2.2, 2.3**
  - [x] 2.4 Write property test `TestProperty_TitleExtractionRoundTrip` in `server/title_test.go`
    - **Property 3: Title extraction round-trip**
    - Generate valid titles (non-empty, no path separators, no null bytes), build filename as `"note_" + title + ".md"`, extract title, verify round-trip
    - **Validates: Requirements 11.4**

- [x] 3. Checkpoint — Ensure proto rename and title extraction work
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. CreateNote security hardening
  - [x] 4.1 Fix path traversal in `server/createNote.go` — use validated `dirPath` from `pathutil.ValidatePath` for `os.MkdirAll` and file path construction instead of raw `req.GetParentDir()`
    - _Requirements: 1.1, 1.2, 1.3, 12.1_
  - [x] 4.2 Add null byte validation for title in `server/createNote.go` — reject titles containing null bytes with `CodeInvalidArgument`
    - _Requirements: 5.1, 5.2_
  - [x] 4.3 Add duplicate detection in `server/createNote.go` — check if file already exists before writing, return `CodeAlreadyExists` if so
    - _Requirements: 4.1, 4.2, 4.3_
  - [x] 4.4 Write property test `TestProperty_CreateNotePathCanonicalization` in `server/createNote_property_test.go`
    - **Property 1: CreateNote path canonicalization**
    - Generate valid directory paths and equivalent unclean forms, verify identical outcomes
    - **Validates: Requirements 1.1, 1.3**
  - [x] 4.5 Write property test `TestProperty_NullByteTitlesRejected` in `server/createNote_property_test.go`
    - **Property 6: Null byte titles are rejected**
    - Generate strings containing null bytes, call CreateNote, verify `CodeInvalidArgument`
    - **Validates: Requirements 5.1**
  - [x] 4.6 Write property test `TestProperty_CreateNoteDuplicateDetection` in `server/createNote_property_test.go`
    - **Property 5: CreateNote duplicate detection**
    - Generate valid titles, call CreateNote twice, verify first succeeds and second returns `CodeAlreadyExists`
    - **Validates: Requirements 4.1, 4.2**

- [x] 5. UpdateNote existence check and cleanup
  - [x] 5.1 Add existence check in `server/updateNote.go` — return `CodeNotFound` if file does not exist, before writing
    - _Requirements: 3.1, 3.2_
  - [x] 5.2 Remove no-op `fullPath := absPath` assignment in `server/updateNote.go` — use `absPath` directly
    - _Requirements: 14.2_
  - [x] 5.3 Write property test `TestProperty_UpdateNoteRejectsNonExistent` in `server/updateNote_property_test.go`
    - **Property 4: UpdateNote rejects non-existent files**
    - Generate random non-existent file paths, call UpdateNote, verify `CodeNotFound` and no file created
    - **Validates: Requirements 3.1, 3.2**

- [x] 6. GetNote cleanup
  - [x] 6.1 Remove no-op `fullPath := absPath` assignment in `server/getNote.go` — use `absPath` directly
    - _Requirements: 14.1_

- [x] 7. Consolidate path validation across services
  - [x] 7.1 Add `ValidateParentDir` helper to `pathutil/pathutil.go` for directory path validation (allows data directory root)
    - _Requirements: 12.1, 12.2, 12.3_
  - [x] 7.2 Replace inline path validation in `server/listNotes.go` with `pathutil.ValidateParentDir`
    - _Requirements: 12.2_
  - [x] 7.3 Replace inline path validation in `file/list_files.go`, `file/create_folder.go`, `file/update_folder.go`, `file/delete_folder.go` with `pathutil.ValidatePath` or `pathutil.ValidateParentDir`
    - _Requirements: 12.3_
  - [x] 7.4 Replace inline path validation in `tasks/create_task_list.go` with `pathutil.ValidatePath` or `pathutil.ValidateParentDir`
    - _Requirements: 12.4_
  - [x] 7.5 Verify all existing tests for affected handlers still pass
    - _Requirements: 12.5_

- [x] 8. Shared task validation
  - [x] 8.1 Create `tasks/validate.go` with `validateTasks` function — due_date/recurrence mutual exclusion and RRULE validation
    - _Requirements: 13.1_
  - [x] 8.2 Update `tasks/create_task_list.go` and `tasks/update_task_list.go` to call `validateTasks` instead of inline validation
    - _Requirements: 13.2_
  - [x] 8.3 Verify existing tests for CreateTaskList and UpdateTaskList still pass
    - _Requirements: 13.3_

- [x] 9. Checkpoint — Ensure all refactoring and security fixes work
  - Ensure all tests pass, ask the user if questions arise.

- [x] 10. Token type differentiation
  - [x] 10.1 Add `TokenType string` field (JSON tag `"type"`) to `TokenClaims` in `auth/token_service.go`
    - _Requirements: 6.1, 6.2_
  - [x] 10.2 Update `GenerateAccessToken` to set `TokenType: "access"` and `GenerateRefreshToken` to set `TokenType: "refresh"`
    - _Requirements: 6.1, 6.2_
  - [x] 10.3 Update auth interceptor in `auth/interceptor.go` to reject tokens where `type` claim is not `"access"`
    - _Requirements: 6.3_
  - [x] 10.4 Update `RefreshToken` handler in `auth/auth_server.go` to reject tokens where `type` claim is not `"refresh"`
    - _Requirements: 7.1, 7.2_
  - [x] 10.5 Fix RefreshToken error message — change `"invalid credentials"` to `"invalid or expired refresh token"` in `auth/auth_server.go`
    - _Requirements: 9.1_
  - [x] 10.6 Update existing auth tests to account for new token type claim
    - _Requirements: 6.4, 7.3_
  - [x] 10.7 Write property test `TestProperty_TokenTypeRoundTrip` in `auth/token_service_test.go`
    - **Property 7: Token type round-trip**
    - Generate random usernames, generate access and refresh tokens, parse them, verify type claims
    - **Validates: Requirements 6.1, 6.2, 6.4**
  - [x] 10.8 Write property test `TestProperty_InterceptorRejectsRefreshTokens` in `auth/interceptor_test.go`
    - **Property 8: Auth interceptor rejects non-access tokens**
    - Generate random usernames, generate refresh tokens, use as bearer token, verify `CodeUnauthenticated`
    - **Validates: Requirements 6.3**
  - [x] 10.9 Write property test `TestProperty_RefreshEndpointEnforcesTokenType` in `auth/auth_server_test.go`
    - **Property 9: RefreshToken endpoint enforces token type**
    - Generate access tokens, call RefreshToken, verify rejection; generate refresh tokens, verify success
    - **Validates: Requirements 7.1, 7.2**

- [ ] 11. Username enumeration prevention
  - [ ] 11.1 Update `auth/user_store.go` — change `getUser` to return generic `"invalid credentials"` error instead of including the username
    - Ensure both "user not found" and "wrong password" paths return identical error messages
    - _Requirements: 18.1, 18.2, 18.3_
  - [ ] 11.2 Write property test `TestProperty_AuthErrorUniformity` in `auth/user_store_test.go`
    - **Property 10: Authentication error uniformity**
    - Generate random username/password pairs, authenticate with non-existent users and wrong passwords, verify all error messages are identical and contain no username
    - **Validates: Requirements 18.1, 18.2, 18.3**

- [ ] 12. Checkpoint — Ensure auth changes work
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 13. Operational improvements
  - [ ] 13.1 Replace German log message in `main.go` — change `"ConnectRPC Server läuft auf"` to `"ConnectRPC Server listening on"`
    - _Requirements: 10.1_
  - [ ] 13.2 Add fsync before rename in `atomicwrite/atomicwrite.go` — call `tmp.Sync()` after writing and before closing, remove temp file on sync failure
    - _Requirements: 17.1, 17.2, 17.3_
  - [ ] 13.3 Add request size limit middleware in `main.go` — wrap handler with `http.MaxBytesReader`, configurable via `MAX_REQUEST_BODY_BYTES` env var (default 4MB)
    - _Requirements: 16.1, 16.2, 16.3_
  - [ ] 13.4 Add graceful shutdown in `main.go` — listen for SIGINT/SIGTERM, call `srv.Shutdown` with configurable timeout via `SHUTDOWN_TIMEOUT_SECONDS` env var (default 30s), log shutdown start and completion
    - _Requirements: 15.1, 15.2, 15.3_

- [ ] 14. Final checkpoint — Ensure all tests pass
  - Run `go build ./...` and `go test ./...` to verify the entire project compiles and all tests pass.
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Property tests use the `pgregory.net/rapid` library already in the project
- Excluded from scope: Alpine image pinning and login rate limiting (per user request)
- Proto field rename (Task 1) is done first since it affects multiple subsequent tasks
- Checkpoints are placed after major groups of related changes
