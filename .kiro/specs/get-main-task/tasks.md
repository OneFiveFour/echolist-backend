# Tasks

## Task 1: Add Proto Definition

- [x] 1.1 Add `GetMainTask` RPC to `TaskListService` in `proto/tasks/v1/tasks.proto`
- [x] 1.2 Add `GetMainTaskRequest` message with `string id = 1` field
- [x] 1.3 Add `GetMainTaskResponse` message with `MainTask main_task = 1` field
- [x] 1.4 Regenerate Go code with `buf generate`

## Task 2: Add Database Method

- [x] 2.1 Add `GetMainTask(id string) (TaskRow, []TaskRow, error)` method to `database/task_lists.go`
- [x] 2.2 Query fetches the main task row (WHERE id = ? AND task_list_id IS NOT NULL) and its subtask rows (WHERE parent_task_id = ? ORDER BY position)
- [x] 2.3 Return `ErrNotFound` if the ID does not exist or is not a main task

## Task 3: Add RPC Handler

- [x] 3.1 Create `tasks/get_main_task.go` implementing `GetMainTask` on `TaskServer`
- [x] 3.2 Validate request ID with `common.ValidateUuidV4`
- [x] 3.3 Call `s.db.GetMainTask(id)` and map `ErrNotFound` to `connect.CodeNotFound`
- [x] 3.4 Convert `TaskRow` + subtask `[]TaskRow` to domain `MainTask` using a helper function
- [x] 3.5 Convert domain `MainTask` to proto and return `GetMainTaskResponse`

## Task 4: Add Unit Tests

- [x] 4.1 Add `TestGetMainTask_Success` example-based test in `tasks/crud_test.go`
- [x] 4.2 Add `TestGetMainTask_NotFound` example-based test
- [x] 4.3 Add `TestGetMainTask_SubtaskIdReturnsNotFound` example-based test

## Task 5: Add Property-Based Tests

- [x] 5.1 Add `TestProperty_GetMainTaskRoundTrip` to `tasks/property_test.go` — Feature: get-main-task, Property 1
- [x] 5.2 Add `TestProperty_GetMainTaskNotFound` — Feature: get-main-task, Property 2
- [x] 5.3 Add `TestProperty_GetMainTaskInvalidUUID` — Feature: get-main-task, Property 3
- [x] 5.4 Add `TestProperty_GetMainTaskIsolation` — Feature: get-main-task, Property 4

## Task 6: Verify

- [x] 6.1 Run `go build ./...` to confirm compilation
- [x] 6.2 Run `go test ./tasks/...` to confirm all tests pass
