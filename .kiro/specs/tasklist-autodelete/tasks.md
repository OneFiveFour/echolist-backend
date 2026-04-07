# Implementation Plan: TaskList AutoDelete

## Overview

Add an `is_auto_delete` boolean flag to task lists. The implementation proceeds bottom-up: proto changes → code generation → registry refactor → filter function → handler updates → property tests. Each step builds on the previous one so there is no orphaned code.

## Tasks

- [x] 1. Proto changes and code generation
  - [x] 1.1 Add `is_auto_delete` fields to proto messages
    - Add `bool is_auto_delete = 6` to `TaskList` message in `proto/tasks/v1/tasks.proto`
    - Add `bool is_auto_delete = 4` to `CreateTaskListRequest` message
    - Add `bool is_auto_delete = 4` to `UpdateTaskListRequest` message
    - _Requirements: 1.1, 1.2, 1.3_
  - [x] 1.2 Regenerate Go protobuf code
    - Run `buf generate` from the `proto/` directory to regenerate `proto/gen/tasks/v1/tasks.pb.go` and connect files
    - _Requirements: 1.1, 1.2, 1.3_

- [x] 2. Refactor registry to use structured entries
  - [x] 2.1 Define `registryEntry` struct and update registry functions
    - Add `registryEntry` struct with `FilePath string` and `IsAutoDelete bool` JSON fields in `tasks/registry.go`
    - Change `registryRead` return type from `map[string]string` to `map[string]registryEntry`
    - Change `registryWrite` parameter from `map[string]string` to `map[string]registryEntry`
    - Update `registryLookup` to return `(registryEntry, bool, error)` instead of `(string, bool, error)`
    - Update `registryAdd` to accept `registryEntry` instead of a plain string
    - Update `registryRemove` to work with `map[string]registryEntry`
    - _Requirements: 2.4_
  - [x] 2.2 Fix all compilation errors from registry signature changes
    - Update `create_task_list.go` to pass `registryEntry` to `registryAdd`
    - Update `get_task_list.go` to read `filePath` from the returned `registryEntry`
    - Update `update_task_list.go` to read `filePath` from the returned `registryEntry` and pass `registryEntry` to `registryAdd`
    - Update `delete_task_list.go` to read `filePath` from the returned `registryEntry`
    - Update `list_task_lists.go` to iterate over `map[string]registryEntry` for the reverse map
    - _Requirements: 2.4_

- [x] 3. Checkpoint
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. Implement `filterAutoDeleted` pure function
  - [x] 4.1 Add `filterAutoDeleted` function in `tasks/update_task_list.go`
    - Implement `filterAutoDeleted(tasks []MainTask) []MainTask`
    - Remove MainTasks where `Done == true` and `Recurrence == ""` (non-recurring done tasks), along with all their SubTasks
    - Remove SubTasks where `Done == true` from surviving MainTasks
    - Return a new slice; do not mutate the input
    - _Requirements: 3.1, 4.1, 6.2_
  - [x] 4.2 Write property test: Property 2 — AutoDelete removes done non-recurring MainTasks
    - **Property 2: AutoDelete removes done non-recurring MainTasks**
    - Generate random `[]MainTask` with mix of done/open, recurring/non-recurring
    - Apply `filterAutoDeleted`; assert no done non-recurring MainTasks remain and none of their SubTasks are present
    - **Validates: Requirements 3.1, 6.2, 7.1**
  - [x] 4.3 Write property test: Property 5 — AutoDelete removes done SubTasks
    - **Property 5: AutoDelete removes done SubTasks**
    - Generate MainTasks with mix of done/open SubTasks; apply `filterAutoDeleted`
    - Assert no done SubTasks remain on surviving MainTasks; open SubTasks are retained
    - **Validates: Requirements 4.1**

- [x] 5. Update `buildTaskList` helper and RPC handlers
  - [x] 5.1 Add `isAutoDelete` parameter to `buildTaskList` in `tasks/task_server.go`
    - Add `isAutoDelete bool` parameter to `buildTaskList`
    - Set `IsAutoDelete` on the returned `pb.TaskList`
    - Update all existing call sites to pass the `isAutoDelete` value
    - _Requirements: 1.4, 7.3_
  - [x] 5.2 Update `CreateTaskList` handler in `tasks/create_task_list.go`
    - Read `req.GetIsAutoDelete()` and pass it into the `registryEntry` when calling `registryAdd`
    - Pass `isAutoDelete` to `buildTaskList` in the response
    - _Requirements: 2.1, 2.2_
  - [x] 5.3 Update `GetTaskList` handler in `tasks/get_task_list.go`
    - Read `isAutoDelete` from the `registryEntry` returned by `registryLookup`
    - Pass `isAutoDelete` to `buildTaskList` in the response
    - _Requirements: 1.4_
  - [x] 5.4 Update `ListTaskLists` handler in `tasks/list_task_lists.go`
    - Build reverse map from `map[string]registryEntry` to also carry `isAutoDelete`
    - Populate `IsAutoDelete` on each `pb.TaskList` in the response
    - _Requirements: 1.4_
  - [x] 5.5 Update `UpdateTaskList` handler in `tasks/update_task_list.go`
    - Read `req.GetIsAutoDelete()` from the request
    - After recurrence advancement, call `filterAutoDeleted` on the task list if `isAutoDelete` is true
    - Update the registry entry with the new `isAutoDelete` value via `registryAdd`
    - Pass `isAutoDelete` to `buildTaskList` in the response
    - _Requirements: 2.3, 3.1, 3.2, 3.3, 4.1, 4.2, 7.1, 7.2, 7.3_
  - [x] 5.6 Update `DeleteTaskList` handler in `tasks/delete_task_list.go`
    - Read `filePath` from the `registryEntry` returned by `registryLookup` (if not already done in task 2.2)
    - No AutoDelete logic change needed; just ensure it compiles with the new registry types
    - _Requirements: (no new requirements; existing behavior preserved)_

- [x] 6. Checkpoint
  - Ensure all tests pass, ask the user if questions arise.

- [x] 7. Property-based and unit tests for end-to-end behavior
  - [x] 7.1 Write property test: Property 1 — AutoDelete flag round-trip
    - **Property 1: AutoDelete flag round-trip**
    - Generate random bool, create a task list with that value, read it back via GetTaskList, assert `is_auto_delete` matches
    - **Validates: Requirements 1.4, 2.1, 2.2, 2.3, 2.5, 7.3**
  - [x] 7.2 Write property test: Property 3 — AutoDelete disabled retains all tasks
    - **Property 3: AutoDelete disabled retains all tasks**
    - Generate random task lists with random done states; update with AutoDelete off; assert all tasks present with done status preserved
    - **Validates: Requirements 3.2, 4.2, 7.2**
  - [x] 7.3 Write property test: Property 4 — AutoDelete advances recurring tasks
    - **Property 4: AutoDelete advances recurring tasks instead of deleting**
    - Generate recurring tasks marked done; apply update with AutoDelete on; assert task present, `done = false`, due date advanced
    - **Validates: Requirements 3.3, 6.1**
  - [x] 7.4 Write property test: Property 6 — Manual deletion cascades regardless of AutoDelete
    - **Property 6: Manual deletion cascades regardless of AutoDelete**
    - Generate two task lists (AutoDelete on and off), omit same tasks from both update requests; assert identical removal results
    - **Validates: Requirements 5.1, 5.2**

- [x] 8. Final checkpoint
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks are mandatory
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties from the design document using `pgregory.net/rapid`
- Proto code generation (task 1.2) must run before any Go compilation
- Registry refactor (task 2) must complete before handler updates (task 5) to avoid intermediate compile errors
