# Requirements Document

## Introduction

This spec addresses 18 findings from a code review of the echolist-backend project. The findings span security vulnerabilities (path traversal, panic-able slicing, silent file creation, missing duplicate checks, null byte injection, token type confusion), API consistency issues (field naming, error messages), a German log message, code duplication (title extraction, path validation, task validation), and operational improvements (no-op assignments, graceful shutdown, request size limits, fsync in atomic writes, username enumeration). The goal is to harden the codebase against the identified security issues, improve API consistency, eliminate duplication, and apply operational best practices.

## Glossary

- **NoteService**: The Connect/gRPC service handling CRUD operations for notes (`server/` package).
- **TaskService**: The Connect/gRPC service handling CRUD operations for task lists (`tasks/` package).
- **FileService**: The Connect/gRPC service handling directory browsing and file listing (`file/` package).
- **AuthService**: The Connect/gRPC service handling login and token refresh (`auth/` package).
- **TokenService**: The component responsible for generating and validating JWT access and refresh tokens (`auth/token_service.go`).
- **UserStore**: The component managing file-based user credential storage (`auth/user_store.go`).
- **pathutil**: The shared utility package providing `ValidatePath` and `IsSubPath` for path traversal protection.
- **atomicwrite**: The shared utility package providing atomic file writes via temp file and rename (`atomicwrite/atomicwrite.go`).
- **Connect_Error_Code**: A structured error code from the Connect RPC framework (e.g., `CodeNotFound`, `CodeInvalidArgument`, `CodeAlreadyExists`).
- **Path_Traversal**: An attack where a malicious relative path escapes the intended data directory.
- **Token_Type**: A claim in a JWT that distinguishes access tokens from refresh tokens (e.g., `"type": "access"` vs `"type": "refresh"`).
- **Title_Extraction**: The logic that derives a human-readable title from a note filename by stripping the `note_` prefix and `.md` suffix.

## Requirements

### Requirement 1: Fix Path Traversal in CreateNote

**User Story:** As a system administrator, I want CreateNote to use the validated path for all file operations, so that path traversal attacks cannot bypass validation.

#### Acceptance Criteria

1. WHEN a CreateNote request is received, THE NoteService SHALL use the validated and cleaned directory path for directory creation and file writing, not the raw request path.
2. WHEN a CreateNote request contains a `path` that would resolve outside the data directory, THE NoteService SHALL return a Connect error with code `CodeInvalidArgument` and SHALL NOT create any files or directories.
3. FOR ALL CreateNote requests with valid paths, THE NoteService SHALL produce identical results whether the path is provided in clean or unclean form (e.g., `foo/../bar` and `bar` produce the same outcome).

### Requirement 2: Prevent Panic on Short Filenames in Note Title Extraction

**User Story:** As a developer, I want note title extraction to handle short filenames safely, so that the server does not panic on unexpected file names.

#### Acceptance Criteria

1. WHEN GetNote encounters a file whose name is shorter than 3 characters, THE NoteService SHALL return a Connect error with code `CodeInternal` instead of panicking.
2. WHEN UpdateNote encounters a file whose name is shorter than 3 characters, THE NoteService SHALL return a Connect error with code `CodeInternal` instead of panicking.
3. FOR ALL filenames of any length, THE Title_Extraction logic SHALL execute without causing a runtime panic.

### Requirement 3: Reject Updates to Non-Existent Notes

**User Story:** As a client of the NoteService, I want UpdateNote to return an error when the target note does not exist, so that I can distinguish between creating and updating notes.

#### Acceptance Criteria

1. WHEN an UpdateNote request references a file path that does not exist, THE NoteService SHALL return a Connect error with code `CodeNotFound`.
2. WHEN an UpdateNote request references a file path that does not exist, THE NoteService SHALL NOT create a new file.

### Requirement 4: Duplicate Detection in CreateNote

**User Story:** As a client of the NoteService, I want CreateNote to reject duplicate note titles in the same directory, so that notes are not silently overwritten.

#### Acceptance Criteria

1. WHEN a CreateNote request specifies a title that already exists as a note file in the target directory, THE NoteService SHALL return a Connect error with code `CodeAlreadyExists`.
2. WHEN a CreateNote request specifies a title that already exists, THE NoteService SHALL NOT overwrite the existing file.
3. THE NoteService duplicate detection behavior SHALL be consistent with the TaskService CreateTaskList duplicate detection behavior.

### Requirement 5: Null Byte Validation in CreateNote Title

**User Story:** As a security-conscious developer, I want CreateNote to reject titles containing null bytes, so that null byte injection attacks against the file system are prevented.

#### Acceptance Criteria

1. WHEN a CreateNote request contains a title with one or more null bytes, THE NoteService SHALL return a Connect error with code `CodeInvalidArgument` and a message indicating the title must not contain null bytes.
2. THE NoteService title validation SHALL check for null bytes consistently with the FileService `validateName` function.

### Requirement 6: Differentiate Access and Refresh Token Types

**User Story:** As a security-conscious developer, I want access tokens and refresh tokens to carry distinct type claims, so that a refresh token cannot be used as an access token and vice versa.

#### Acceptance Criteria

1. WHEN the TokenService generates an access token, THE TokenService SHALL include a `type` claim with the value `"access"` in the JWT payload.
2. WHEN the TokenService generates a refresh token, THE TokenService SHALL include a `type` claim with the value `"refresh"` in the JWT payload.
3. WHEN the AuthInterceptor validates a token for protected endpoints, THE AuthInterceptor SHALL reject tokens whose `type` claim is not `"access"`.
4. FOR ALL generated tokens, parsing then inspecting the `type` claim SHALL return the token type that was originally requested (round-trip property).

### Requirement 7: Validate Token Type in RefreshToken Endpoint

**User Story:** As a security-conscious developer, I want the RefreshToken endpoint to only accept refresh tokens, so that access tokens cannot be used to obtain new access tokens.

#### Acceptance Criteria

1. WHEN the RefreshToken endpoint receives a token whose `type` claim is `"access"`, THE AuthService SHALL return a Connect error with code `CodeUnauthenticated`.
2. WHEN the RefreshToken endpoint receives a token whose `type` claim is `"refresh"`, THE AuthService SHALL generate and return a new access token.
3. WHEN the RefreshToken endpoint receives an expired refresh token, THE AuthService SHALL return a Connect error with code `CodeUnauthenticated`.

### Requirement 8: Consistent Path Field Naming Across Services

**User Story:** As a client developer, I want consistent field naming for directory paths across all services, so that the API is predictable and easy to use.

#### Acceptance Criteria

1. THE NoteService proto messages that accept a directory path for listing or creation SHALL use the field name `parent_dir`, and the FileService and TaskService proto messages that accept a directory path SHALL also use `parent_dir` for consistency.
2. WHEN the field is renamed in the proto definition, THE NoteService handler and all associated tests SHALL be updated to use the new field name.
3. THE renamed field SHALL preserve the same semantics as the original field (a relative directory path within the data directory).

### Requirement 9: Correct Error Message in RefreshToken Endpoint

**User Story:** As a client developer, I want the RefreshToken endpoint to return a descriptive error message, so that I can distinguish authentication failures from token refresh failures.

#### Acceptance Criteria

1. WHEN the RefreshToken endpoint receives an invalid or expired token, THE AuthService SHALL return an error message containing "invalid or expired refresh token" instead of "invalid credentials".

### Requirement 10: Replace German Log Message with English

**User Story:** As a developer reading server logs, I want all log messages to be in English, so that the logs are consistent and accessible to all team members.

#### Acceptance Criteria

1. THE main.go startup log message SHALL read "ConnectRPC Server listening on" followed by the address, replacing the German text "ConnectRPC Server läuft auf".

### Requirement 11: Extract Shared Title Extraction Logic for Notes

**User Story:** As a maintainer of the codebase, I want a single shared function for extracting note titles from filenames, so that the logic is consistent and maintained in one place.

#### Acceptance Criteria

1. THE NoteService SHALL contain exactly one implementation of the Title_Extraction logic, located in a shared helper function.
2. THE GetNote, ListNotes, and UpdateNote handlers SHALL use the shared Title_Extraction function instead of inline title extraction.
3. WHEN the shared Title_Extraction function is introduced, THE existing tests for GetNote, ListNotes, and UpdateNote SHALL continue to pass.
4. FOR ALL valid note filenames, extracting the title then reconstructing the filename SHALL produce the original filename (round-trip property).

### Requirement 12: Consolidate Path Validation Boilerplate

**User Story:** As a maintainer of the codebase, I want all handlers to use `pathutil.ValidatePath` for path validation, so that the validation logic is consistent and not duplicated inline.

#### Acceptance Criteria

1. THE CreateNote handler SHALL use `pathutil.ValidatePath` or `pathutil.IsSubPath` for path validation instead of inline path traversal checks.
2. THE ListNotes handler SHALL use `pathutil.ValidatePath` or `pathutil.IsSubPath` for path validation instead of inline path traversal checks.
3. THE FileService handlers (ListFiles, CreateFolder, UpdateFolder, DeleteFolder) SHALL use `pathutil.ValidatePath` or `pathutil.IsSubPath` for path validation instead of inline path traversal checks.
4. THE CreateTaskList handler SHALL use `pathutil.ValidatePath` or `pathutil.IsSubPath` for path validation instead of inline path traversal checks.
5. WHEN path validation is consolidated, THE existing tests for all affected handlers SHALL continue to pass.

### Requirement 13: Extract Shared Task Validation Logic

**User Story:** As a maintainer of the codebase, I want task validation (due_date/recurrence mutual exclusion and RRULE validation) to be implemented once, so that CreateTaskList and UpdateTaskList stay consistent.

#### Acceptance Criteria

1. THE TaskService SHALL contain exactly one implementation of the task validation logic (due_date/recurrence mutual exclusion and RRULE validation), located in a shared helper function.
2. THE CreateTaskList and UpdateTaskList handlers SHALL use the shared task validation function.
3. WHEN the shared task validation function is introduced, THE existing tests for CreateTaskList and UpdateTaskList SHALL continue to pass.

### Requirement 14: Remove No-Op fullPath Assignments

**User Story:** As a maintainer of the codebase, I want to remove no-op variable assignments, so that the code is clear and free of dead assignments.

#### Acceptance Criteria

1. THE GetNote handler SHALL use the `absPath` variable directly instead of assigning it to a redundant `fullPath` variable.
2. THE UpdateNote handler SHALL use the `absPath` variable directly instead of assigning it to a redundant `fullPath` variable.

### Requirement 15: Graceful Shutdown in main.go

**User Story:** As a system administrator, I want the server to shut down gracefully on SIGINT/SIGTERM, so that in-flight requests complete before the process exits.

#### Acceptance Criteria

1. WHEN the server process receives a SIGINT or SIGTERM signal, THE server SHALL stop accepting new connections and wait for in-flight requests to complete before exiting.
2. WHEN the graceful shutdown period exceeds a configurable timeout, THE server SHALL force-close remaining connections and exit.
3. WHEN the server shuts down gracefully, THE server SHALL log a message indicating shutdown has started and completed.

### Requirement 16: Request Size Limits

**User Story:** As a system administrator, I want the server to enforce request size limits, so that excessively large requests cannot exhaust server memory.

#### Acceptance Criteria

1. THE server SHALL enforce a maximum request body size limit on all incoming requests.
2. WHEN a request exceeds the maximum body size, THE server SHALL reject the request with an appropriate error before reading the full body into memory.
3. THE maximum request body size SHALL be configurable via an environment variable, with a sensible default value.

### Requirement 17: Add fsync Before Rename in atomicwrite

**User Story:** As a developer, I want atomic writes to fsync the temp file before renaming, so that data is durable on disk and not lost on power failure.

#### Acceptance Criteria

1. WHEN writing a file atomically, THE atomicwrite.File function SHALL call `fsync` (via `file.Sync()`) on the temporary file after writing and before closing.
2. WHEN the `fsync` call fails, THE atomicwrite.File function SHALL remove the temporary file and return the error.
3. WHEN the `fsync` call succeeds, THE atomicwrite.File function SHALL proceed with the rename operation.

### Requirement 18: Prevent Username Enumeration in UserStore

**User Story:** As a security-conscious developer, I want authentication errors to not reveal whether a username exists, so that attackers cannot enumerate valid usernames.

#### Acceptance Criteria

1. WHEN authentication fails due to a non-existent username, THE UserStore SHALL return a generic "invalid credentials" error without including the username in the error message.
2. WHEN authentication fails due to an incorrect password, THE UserStore SHALL return the same generic "invalid credentials" error.
3. FOR ALL authentication failures regardless of cause, THE error message returned to the caller SHALL be identical.
