# Requirements Document

## Introduction

The TaskListService protobuf API currently duplicates task list fields (`file_path`, `name`, `tasks`, `updated_at`) across individual response messages (`CreateTaskListResponse`, `GetTaskListResponse`, `UpdateTaskListResponse`) and uses a separate, incomplete `TaskListEntry` message for `ListTaskListsResponse`. The NoteService already follows a cleaner pattern where a single `Note` message encapsulates all resource fields and is reused across all responses. This spec unifies the TaskListService to follow the same pattern: define a single `TaskList` message containing all current and future task list fields, embed it in every request/response that deals with a task list resource, and remove the now-redundant `TaskListEntry` message.

## Glossary

- **Backend**: The Go server application exposing Connect/gRPC services defined via protobuf
- **TaskListService**: The protobuf service responsible for task list CRUD operations, defined in `proto/tasks/v1/tasks.proto`
- **TaskList**: The new unified protobuf message representing a task list resource with `file_path`, `name`, `tasks`, and `updated_at` fields
- **TaskListEntry**: The current protobuf message used in `ListTaskListsResponse` that only contains `file_path`, `name`, and `updated_at` (missing `tasks`)
- **Note**: The existing unified protobuf message in `proto/notes/v1/notes.proto` that serves as the pattern to follow
- **MainTask**: The existing protobuf message representing a top-level task with description, status, due date, recurrence, and subtasks
- **Connect_Handler**: The generated Go interface and registration function produced by the Connect framework from a protobuf service definition

## Requirements

### Requirement 1: Define a TaskList protobuf message

**User Story:** As a developer, I want a single `TaskList` message that encapsulates all task list fields, so that the task list resource has a canonical representation consistent with the `Note` message pattern.

#### Acceptance Criteria

1. THE Backend SHALL define a `TaskList` protobuf message in `proto/tasks/v1/tasks.proto` with the fields `string file_path = 1`, `string name = 2`, `repeated MainTask tasks = 3`, and `int64 updated_at = 4`
2. THE TaskList message SHALL contain all fields that currently appear in `CreateTaskListResponse`, `GetTaskListResponse`, and `UpdateTaskListResponse`
3. THE TaskList message SHALL serve as the single source of truth for task list resource fields, so that future field additions only require changes to the `TaskList` message

### Requirement 2: Embed TaskList message in CreateTaskList response

**User Story:** As a developer, I want `CreateTaskListResponse` to embed the `TaskList` message instead of duplicating its fields, so that the response payload is DRY and consistent with the `Note` pattern.

#### Acceptance Criteria

1. THE Backend SHALL redefine `CreateTaskListResponse` with a single `TaskList task_list = 1` field replacing the individual `file_path`, `name`, `tasks`, and `updated_at` fields
2. THE Backend SHALL update the CreateTaskList handler to populate and return a `TaskList` message inside `CreateTaskListResponse`

### Requirement 3: Embed TaskList message in GetTaskList response

**User Story:** As a developer, I want `GetTaskListResponse` to embed the `TaskList` message instead of duplicating its fields, so that the response payload is DRY and consistent with the `Note` pattern.

#### Acceptance Criteria

1. THE Backend SHALL redefine `GetTaskListResponse` with a single `TaskList task_list = 1` field replacing the individual `file_path`, `name`, `tasks`, and `updated_at` fields
2. THE Backend SHALL update the GetTaskList handler to populate and return a `TaskList` message inside `GetTaskListResponse`

### Requirement 4: Embed TaskList message in UpdateTaskList response

**User Story:** As a developer, I want `UpdateTaskListResponse` to embed the `TaskList` message instead of duplicating its fields, so that the response payload is DRY and consistent with the `Note` pattern.

#### Acceptance Criteria

1. THE Backend SHALL redefine `UpdateTaskListResponse` with a single `TaskList task_list = 1` field replacing the individual `file_path`, `name`, `tasks`, and `updated_at` fields
2. THE Backend SHALL update the UpdateTaskList handler to populate and return a `TaskList` message inside `UpdateTaskListResponse`

### Requirement 5: Replace TaskListEntry with TaskList in ListTaskLists response

**User Story:** As a developer, I want `ListTaskListsResponse` to return a list of full `TaskList` messages instead of the incomplete `TaskListEntry` messages, so that listing task lists provides all resource information without requiring follow-up GetTaskList calls.

#### Acceptance Criteria

1. THE Backend SHALL redefine `ListTaskListsResponse` to use `repeated TaskList task_lists = 1` replacing `repeated TaskListEntry task_lists = 1`
2. THE Backend SHALL retain the `repeated string entries = 2` field in `ListTaskListsResponse` for folder/file path listing
3. THE Backend SHALL remove the `TaskListEntry` message from `proto/tasks/v1/tasks.proto`
4. THE Backend SHALL update the ListTaskLists handler to populate and return full `TaskList` messages (including `tasks` field) for each task list in the response

### Requirement 6: Update Go handler implementations

**User Story:** As a developer, I want all Go handler implementations to use the new `TaskList` message, so that the server compiles and serves the unified API correctly.

#### Acceptance Criteria

1. THE Backend SHALL regenerate Connect_Handler code from the updated protobuf definition
2. THE Backend SHALL update the CreateTaskList handler to construct a `TaskList` message and wrap it in `CreateTaskListResponse`
3. THE Backend SHALL update the GetTaskList handler to construct a `TaskList` message and wrap it in `GetTaskListResponse`
4. THE Backend SHALL update the ListTaskLists handler to construct `TaskList` messages for each task list and return them in `ListTaskListsResponse`
5. THE Backend SHALL update the UpdateTaskList handler to construct a `TaskList` message and wrap it in `UpdateTaskListResponse`
6. IF any client code references the removed `TaskListEntry` type, THEN THE Backend SHALL update those references to use the `TaskList` message
