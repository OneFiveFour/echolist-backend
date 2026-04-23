# Requirements Document

## Introduction

MainTask entities in echolist-backend are currently pure value types with no unique identifier. Individual tasks within a TaskList are identified only by their position in the `TaskList.tasks` repeated field. This creates problems on the client side when navigating to a detail/settings screen for a specific task (a stable identifier is needed as a navigation parameter and to reload the task's data) and when matching results back to the correct task after the settings screen returns updated values (index-based matching is fragile if the list changes between navigation events).

This feature adds a unique, stable `id` field to each MainTask so that individual tasks can be reliably referenced across reordering, editing, and sibling additions/removals. SubTask receives the same treatment as a secondary priority. Since this is a pre-release change, no backwards compatibility or migration code is required.

## Glossary

- **MainTask**: A top-level task within a TaskList, represented by the `MainTask` protobuf message and the `MainTask` Go struct in the `tasks` package.
- **SubTask**: A child task nested under a MainTask, represented by the `SubTask` protobuf message and the `SubTask` Go struct.
- **TaskList**: A collection of MainTasks persisted as a markdown file on disk, identified by a stable TaskList_ID.
- **TaskListService**: The gRPC/Connect service defined in `tasks.proto` that exposes CRUD operations for task lists.
- **MainTask_ID**: A stable, opaque, synthetic identifier assigned to a MainTask when the backend first processes it. The MainTask_ID remains constant for the lifetime of the MainTask regardless of reordering, editing, or sibling changes.
- **SubTask_ID**: A stable, opaque, synthetic identifier assigned to a SubTask when the backend first processes it. The SubTask_ID remains constant for the lifetime of the SubTask regardless of reordering, editing, or sibling changes.
- **Task_File**: The markdown file on disk that stores the serialized representation of all MainTasks and SubTasks in a TaskList.
- **Parser**: The component (`ParseTaskFile`) that reads a Task_File's byte content and produces a list of MainTask domain objects.
- **Printer**: The component (`PrintTaskFile`) that serializes a list of MainTask domain objects into the Task_File byte format.
- **CreateTaskList_RPC**: The RPC that creates a new task list file and returns the resulting TaskList message.
- **GetTaskList_RPC**: The RPC that retrieves a single task list by identifier.
- **UpdateTaskList_RPC**: The RPC that updates the tasks, title, or settings of an existing task list.
- **ListTaskLists_RPC**: The RPC that returns all task lists under a given parent directory.

## Requirements

### Requirement 1: MainTask_ID Generation

**User Story:** As a client developer, I want every MainTask to have a unique stable ID, so that I can navigate to a task's detail screen and reliably apply results back to the correct task.

#### Acceptance Criteria

1. WHEN a TaskList is created via CreateTaskList_RPC, THE TaskListService SHALL assign a MainTask_ID to each MainTask in the request that does not already have a MainTask_ID.
2. THE TaskListService SHALL generate each MainTask_ID as a version-4 UUID formatted as a lowercase hyphenated string (e.g. `550e8400-e29b-41d4-a716-446655440000`).
3. THE TaskListService SHALL ensure that no two MainTasks within the same TaskList share the same MainTask_ID.

### Requirement 2: MainTask_ID Stability

**User Story:** As a client developer, I want a MainTask's ID to remain constant across updates, so that my navigation state and cached references stay valid.

#### Acceptance Criteria

1. WHEN a TaskList is updated via UpdateTaskList_RPC and the client sends MainTasks with existing MainTask_IDs, THE TaskListService SHALL preserve those MainTask_IDs in the response and persisted data.
2. THE MainTask_ID SHALL remain constant when the MainTask is reordered within the TaskList.
3. THE MainTask_ID SHALL remain constant when the MainTask's description, done status, due date, or recurrence fields are modified.
4. THE MainTask_ID SHALL remain constant when sibling MainTasks are added to or removed from the TaskList.

### Requirement 3: SubTask_ID Generation

**User Story:** As a client developer, I want every SubTask to have a unique stable ID, so that I can reference individual subtasks reliably.

#### Acceptance Criteria

1. WHEN a TaskList is created or updated, THE TaskListService SHALL assign a SubTask_ID to each SubTask that does not already have a SubTask_ID.
2. THE TaskListService SHALL generate each SubTask_ID as a version-4 UUID formatted as a lowercase hyphenated string.
3. THE TaskListService SHALL ensure that no two SubTasks within the same MainTask share the same SubTask_ID.

### Requirement 4: SubTask_ID Stability

**User Story:** As a client developer, I want a SubTask's ID to remain constant across updates, so that subtask references stay valid.

#### Acceptance Criteria

1. WHEN a TaskList is updated via UpdateTaskList_RPC and the client sends SubTasks with existing SubTask_IDs, THE TaskListService SHALL preserve those SubTask_IDs in the response and persisted data.
2. THE SubTask_ID SHALL remain constant when the SubTask is reordered within its parent MainTask.
3. THE SubTask_ID SHALL remain constant when the SubTask's description or done status fields are modified.

### Requirement 5: Protobuf Schema Changes

**User Story:** As a client developer, I want the MainTask and SubTask protobuf messages to include an `id` field, so that stable IDs are available in all API interactions.

#### Acceptance Criteria

1. THE MainTask protobuf message SHALL include a string field named `id` as field number 1, shifting existing fields to accommodate the new field.
2. THE SubTask protobuf message SHALL include a string field named `id` as field number 1, shifting existing fields to accommodate the new field.
3. THE `id` field SHALL be included in all API responses that contain MainTask or SubTask messages (CreateTaskList_RPC, GetTaskList_RPC, UpdateTaskList_RPC, ListTaskLists_RPC).

### Requirement 6: ID Assignment on Create

**User Story:** As a client developer, I want the CreateTaskList response to include assigned IDs for all tasks and subtasks, so that I can immediately use stable IDs for subsequent operations.

#### Acceptance Criteria

1. WHEN a TaskList is successfully created via CreateTaskList_RPC, THE TaskListService SHALL return MainTask messages where each `id` field contains the newly generated MainTask_ID.
2. WHEN a TaskList is successfully created via CreateTaskList_RPC, THE TaskListService SHALL return SubTask messages where each `id` field contains the newly generated SubTask_ID.

### Requirement 7: ID Assignment on Update

**User Story:** As a client developer, I want the UpdateTaskList RPC to preserve existing IDs and assign new IDs to newly added tasks, so that the client can add tasks without generating IDs itself.

#### Acceptance Criteria

1. WHEN an UpdateTaskListRequest contains MainTasks with non-empty `id` fields, THE TaskListService SHALL preserve those MainTask_IDs in the response.
2. WHEN an UpdateTaskListRequest contains MainTasks with empty `id` fields, THE TaskListService SHALL assign new MainTask_IDs to those MainTasks and include the assigned IDs in the response.
3. WHEN an UpdateTaskListRequest contains SubTasks with non-empty `id` fields, THE TaskListService SHALL preserve those SubTask_IDs in the response.
4. WHEN an UpdateTaskListRequest contains SubTasks with empty `id` fields, THE TaskListService SHALL assign new SubTask_IDs to those SubTasks and include the assigned IDs in the response.

### Requirement 8: Task File Persistence

**User Story:** As a backend developer, I want MainTask_IDs and SubTask_IDs to be persisted in the markdown task file, so that IDs survive server restarts and are available when the file is read back.

#### Acceptance Criteria

1. THE Printer SHALL serialize each MainTask_ID into the Task_File format so that the ID is recoverable by the Parser.
2. THE Printer SHALL serialize each SubTask_ID into the Task_File format so that the ID is recoverable by the Parser.
3. THE Parser SHALL parse MainTask_IDs from the Task_File format and populate the corresponding MainTask domain objects.
4. THE Parser SHALL parse SubTask_IDs from the Task_File format and populate the corresponding SubTask domain objects.
5. FOR ALL valid lists of MainTasks with IDs, printing then parsing SHALL produce MainTasks with the same IDs, descriptions, done statuses, due dates, recurrences, and subtasks (round-trip property).

### Requirement 9: ID Validation

**User Story:** As a backend developer, I want incoming task IDs to be validated, so that malformed identifiers are rejected early with clear error messages.

#### Acceptance Criteria

1. WHEN an UpdateTaskListRequest contains a MainTask with a non-empty `id` field that is not a valid version-4 UUID, THE TaskListService SHALL return an InvalidArgument error.
2. WHEN an UpdateTaskListRequest contains a SubTask with a non-empty `id` field that is not a valid version-4 UUID, THE TaskListService SHALL return an InvalidArgument error.
3. THE TaskListService SHALL validate all task and subtask `id` fields before performing any file system operations.

### Requirement 10: Domain Type Changes

**User Story:** As a backend developer, I want the Go domain types to carry the ID field, so that the ID flows through all internal processing without loss.

#### Acceptance Criteria

1. THE MainTask Go struct SHALL include a string field named `ID` that carries the MainTask_ID.
2. THE SubTask Go struct SHALL include a string field named `ID` that carries the SubTask_ID.
3. THE conversion functions between protobuf messages and domain types SHALL map the `id` field bidirectionally without loss.

### Requirement 11: Round-Trip Property

**User Story:** As a developer, I want a round-trip guarantee that creating a task list and then retrieving it by ID produces tasks with the same IDs, so that the ID-based workflow is trustworthy.

#### Acceptance Criteria

1. FOR ALL valid task list titles and task values, creating a TaskList via CreateTaskList_RPC and then retrieving the TaskList via GetTaskList_RPC SHALL produce MainTasks with the same MainTask_IDs that were returned in the CreateTaskList response.
2. FOR ALL valid task list titles and task values, creating a TaskList via CreateTaskList_RPC and then retrieving the TaskList via GetTaskList_RPC SHALL produce SubTasks with the same SubTask_IDs that were returned in the CreateTaskList response.
