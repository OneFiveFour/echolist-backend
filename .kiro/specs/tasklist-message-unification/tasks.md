# Tasks: TaskList Message Unification

## Task 1: Define TaskList protobuf message and update response types
- [x] 1.1 Add `TaskList` message to `proto/tasks/v1/tasks.proto` with fields: `string file_path = 1`, `string name = 2`, `repeated MainTask tasks = 3`, `int64 updated_at = 4`
- [x] 1.2 Redefine `CreateTaskListResponse` to contain a single `TaskList task_list = 1` field
- [x] 1.3 Redefine `GetTaskListResponse` to contain a single `TaskList task_list = 1` field
- [x] 1.4 Redefine `UpdateTaskListResponse` to contain a single `TaskList task_list = 1` field
- [x] 1.5 Redefine `ListTaskListsResponse` to use `repeated TaskList task_lists = 1` (keep `repeated string entries = 2`)
- [x] 1.6 Remove the `TaskListEntry` message from `proto/tasks/v1/tasks.proto`
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 3.1, 4.1, 5.1, 5.2, 5.3_

## Task 2: Regenerate protobuf Go code
- [x] 2.1 Run `buf generate` (or project-specific codegen command) to regenerate `proto/gen/tasks/v1/tasks.pb.go` and `proto/gen/tasks/v1/tasksv1connect/`
  - _Requirements: 6.1_

## Task 3: Add buildTaskList helper function
- [x] 3.1 Add `buildTaskList(filePath, name string, tasks []MainTask, updatedAt int64) *pb.TaskList` helper to `tasks/task_server.go`
  - _Requirements: 1.3_

## Task 4: Update handler implementations
- [x] 4.1 Update `tasks/create_task_list.go` to return `CreateTaskListResponse` with embedded `TaskList` using `buildTaskList`
- [x] 4.2 Update `tasks/get_task_list.go` to return `GetTaskListResponse` with embedded `TaskList` using `buildTaskList`
- [x] 4.3 Update `tasks/update_task_list.go` to return `UpdateTaskListResponse` with embedded `TaskList` using `buildTaskList`
- [x] 4.4 Update `tasks/list_task_lists.go` to return full `TaskList` messages (read and parse each task file) instead of `TaskListEntry`, using `buildTaskList`
  - _Requirements: 2.2, 3.2, 4.2, 5.4, 6.2, 6.3, 6.4, 6.5_

## Task 5: Update existing tests
- [x] 5.1 Update `tasks/task_server_property_test.go` to access response fields through embedded `TaskList` (e.g., `resp.TaskList.FilePath` instead of `resp.FilePath`)
- [x] 5.2 Update any references to `pb.TaskListEntry` in tests to use `pb.TaskList`
  - _Requirements: 6.6_

## Task 6: Write property-based tests for unified TaskList message
- [x] 6.1 Write `TestProperty_CreateGetRoundTripTaskListMessage` — verify create and get both return non-nil `TaskList` with matching fields
  - Feature: tasklist-message-unification, Property 1: Create-then-Get round trip through TaskList message
  - _Requirements: 2.2, 3.2_
- [x] 6.2 Write `TestProperty_UpdateReturnsTaskListMessage` — verify update returns non-nil `TaskList` with correct fields
  - Feature: tasklist-message-unification, Property 2: Update returns correct TaskList message
  - _Requirements: 4.2_
- [x] 6.3 Write `TestProperty_ListReturnsFullTaskListMessages` — verify list returns full `TaskList` messages with tasks populated and entries field correct
  - Feature: tasklist-message-unification, Property 3: ListTaskLists returns full TaskList messages with tasks and entries
  - _Requirements: 5.2, 5.4_

## Task 7: Verify build and all tests pass
- [x] 7.1 Run `go build ./...` to verify the project compiles
- [x] 7.2 Run `go test ./tasks/...` to verify all existing and new tests pass
