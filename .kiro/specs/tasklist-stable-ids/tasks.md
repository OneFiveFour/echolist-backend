# Implementation Plan: TaskList Stable IDs

## Overview

Add a stable UUIDv4 identifier to every task list, persisted in an on-disk JSON registry. Switch Get, Update, and Delete RPCs from `file_path`-based lookup to `id`-based lookup. Since this is pre-release, remove `file_path` from request messages entirely. Implementation language is Go, using `crypto/rand` for UUID generation, `pgregory.net/rapid` for property tests, and the existing `common.File` for atomic writes.

## Tasks

- [x] 1. Update protobuf schema and regenerate Go code
  - [x] 1.1 Modify `proto/tasks/v1/tasks.proto`
    - Add `string id = 1` to `TaskList` message, renumber existing fields (`file_path = 2`, `title = 3`, `tasks = 4`, `updated_at = 5`)
    - Replace `string file_path = 1` with `string id = 1` in `GetTaskListRequest`, `UpdateTaskListRequest`, and `DeleteTaskListRequest`
    - Leave `CreateTaskListRequest` and `ListTaskListsRequest` unchanged
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_
  - [x] 1.2 Regenerate Go protobuf/Connect code
    - Run the protobuf code generation toolchain to produce updated Go stubs
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [x] 2. Implement UUID validation utility (`tasks/uuid.go`)
  - [x] 2.1 Create `tasks/uuid.go` with `validateUuidV4` function
    - Duplicate the `validateUuidV4` function from `notes/uuid.go` into the `tasks` package
    - Use the same regex: `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
    - Return a connect `InvalidArgument` error for invalid IDs
    - _Requirements: 9.1, 9.2_
  - [x] 2.2 Write property test: invalid UUID returns InvalidArgument
    - **Property 8: Invalid UUID returns InvalidArgument**
    - **Validates: Requirements 9.1, 9.2**
    - Add `invalidUuidGen()` generator for strings that are not valid UUIDv4
    - Tag with `Feature: tasklist-stable-ids, Property 8: Invalid UUID returns InvalidArgument`
  - [x] 2.3 Write unit tests for `validateUuidV4`
    - Test valid UUIDs, empty strings, uppercase UUIDs, wrong version digit, wrong variant nibble
    - _Requirements: 9.1, 9.2_

- [x] 3. Implement ID registry (`tasks/registry.go`)
  - [x] 3.1 Create `tasks/registry.go` with registry functions
    - Implement `registryPath`, `registryRead`, `registryWrite`, `registryLookup`, `registryAdd`, `registryRemove`
    - Use `common.File` for atomic writes (temp file + rename)
    - Store registry at `<dataDir>/.tasklist_id_registry.json`
    - Handle missing or empty registry file gracefully (return empty map)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_
  - [x] 3.2 Write unit tests for registry functions
    - Test `registryRead` with missing file, empty file, valid JSON, and malformed JSON
    - Test `registryWrite` creates valid JSON
    - Test `registryAdd` and `registryRemove` round trips
    - Test `registryLookup` for existing and non-existing keys
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5_

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 5. Update CreateTaskList RPC to generate and persist IDs
  - [ ] 5.1 Modify `tasks/create_task_list.go`
    - After writing the task list file, generate a UUIDv4 via `crypto/rand`
    - Call `registryAdd` to persist the ID-to-file-path mapping
    - Acquire file lock first, then registry lock (consistent ordering with notes)
    - Update `buildTaskList` helper signature to accept `id` as first parameter: `buildTaskList(id, filePath, title string, tasks []MainTask, updatedAt int64)`
    - Update all existing callers of `buildTaskList` to pass the `id` (or empty string where appropriate)
    - _Requirements: 1.1, 1.2, 1.3, 2.1, 8.1, 8.2_
  - [ ] 5.2 Write property test: created ID is valid UUIDv4
    - **Property 2: Created ID is valid UUIDv4**
    - **Validates: Requirements 1.2**
    - Tag with `Feature: tasklist-stable-ids, Property 2: Created ID is valid UUIDv4`
  - [ ] 5.3 Write property test: all created IDs are unique
    - **Property 3: All created IDs are unique**
    - **Validates: Requirements 1.3**
    - Tag with `Feature: tasklist-stable-ids, Property 3: All created IDs are unique`

- [ ] 6. Update GetTaskList RPC to resolve by ID
  - [ ] 6.1 Modify `tasks/get_task_list.go`
    - Validate the `id` field with `validateUuidV4`
    - Call `registryLookup` to resolve ID to file path
    - Return `NotFound` if ID is not in registry
    - Pass resolved `id` through to `buildTaskList`
    - _Requirements: 4.1, 4.2, 9.1, 9.2_

- [ ] 7. Update UpdateTaskList RPC to resolve by ID
  - [ ] 7.1 Modify `tasks/update_task_list.go`
    - Validate the `id` field with `validateUuidV4`
    - Call `registryLookup` to resolve ID to file path
    - Return `NotFound` if ID is not in registry
    - Pass resolved `id` through to `buildTaskList`
    - _Requirements: 5.1, 5.2, 9.1, 9.2_
  - [ ] 7.2 Write property test: update by ID preserves the TaskList_ID
    - **Property 4: Update by ID preserves the TaskList_ID**
    - **Validates: Requirements 1.4, 5.1**
    - Tag with `Feature: tasklist-stable-ids, Property 4: Update by ID preserves the TaskList_ID`

- [ ] 8. Update DeleteTaskList RPC to resolve by ID and clean up registry
  - [ ] 8.1 Modify `tasks/delete_task_list.go`
    - Acquire registry lock, resolve ID via `registryLookup`
    - Acquire file lock, delete the file
    - Call `registryRemove` to clean up the registry entry
    - Return `NotFound` if ID is not in registry
    - _Requirements: 6.1, 6.2, 2.2, 9.1, 9.2_
  - [ ] 8.2 Write property test: delete by ID removes file and registry entry
    - **Property 5: Delete by ID removes file and registry entry**
    - **Validates: Requirements 2.2, 6.1**
    - Tag with `Feature: tasklist-stable-ids, Property 5: Delete by ID removes file and registry entry`

- [ ] 9. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 10. Update ListTaskLists RPC to include IDs
  - [ ] 10.1 Modify `tasks/list_task_lists.go`
    - After building task lists from filesystem, read the registry once
    - Build a reverse map (filePath → id) and attach IDs to each TaskList
    - Task lists without a registry entry get an empty `id` field
    - _Requirements: 7.1, 7.2_
  - [ ] 10.2 Write property test: create then list includes the created task list's ID
    - **Property 7: Create then list includes the created task list's ID**
    - **Validates: Requirements 7.1, 10.2**
    - Tag with `Feature: tasklist-stable-ids, Property 7: Create then list includes the created task list's ID`
  - [ ] 10.3 Write unit test: orphan task list file returns empty ID
    - Test that a task list file on disk with no registry entry is listed with an empty `id` field
    - _Requirements: 7.2_

- [ ] 11. Update existing tests and add round-trip property test
  - [ ] 11.1 Update existing test files to use new protobuf fields
    - Update `tasks/task_server_property_test.go` — existing property tests use `FilePath` in Get/Update/Delete requests, switch to `Id`
    - Update `tasks/create_task_list_test.go` — unit tests for create
    - Update `tasks/tasklist_message_property_test.go` — message property tests
    - Update `tasks/validate_limits_test.go` — validation tests
    - Update `tasks/interface_verification_test.go` — interface tests
    - _Requirements: 3.2, 3.3, 3.4, 3.5_
  - [ ] 11.2 Write property test: create-then-get round trip with ID
    - **Property 1: Create-then-get round trip**
    - **Validates: Requirements 1.1, 2.1, 4.1, 8.1, 8.2, 10.1**
    - Tag with `Feature: tasklist-stable-ids, Property 1: Create-then-get round trip`
  - [ ] 11.3 Write property test: non-existent ID returns NotFound
    - **Property 6: Non-existent ID returns NotFound**
    - **Validates: Requirements 4.2, 5.2, 6.2**
    - Add `uuidV4Gen()` generator for valid UUIDv4 strings
    - Tag with `Feature: tasklist-stable-ids, Property 6: Non-existent ID returns NotFound`
  - [ ] 11.4 Write property test: invalid UUID rejected by Get/Update/Delete RPCs
    - **Property 8: Invalid UUID returns InvalidArgument** (RPC-level variant)
    - **Validates: Requirements 9.1, 9.2**
    - Reuse `invalidUuidGen()` generator
    - Tag with `Feature: tasklist-stable-ids, Property 8: Invalid UUID returns InvalidArgument`

- [ ] 12. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks are mandatory — none are marked optional
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document
- Unit tests validate specific examples and edge cases
- Use `Id` not `ID` in Go identifiers per project convention
- `id` is field number 1 in all protobuf messages
