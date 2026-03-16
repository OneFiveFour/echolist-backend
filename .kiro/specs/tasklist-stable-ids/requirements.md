# Requirements Document

## Introduction

TaskLists in echolist-backend currently use a composite natural key of `parent_dir + title` (materialized as `file_path`). This mirrors the same problem that was solved for Notes in the `note-stable-ids` spec: cache invalidation on rename requires atomic updates to every reference, navigation state becomes stale when a task list is renamed or moved, conflict resolution for same-titled task lists created on different devices is ambiguous, and passing a two-part key through ViewModel → Repository → Cache → Network adds coupling.

This feature adds a stable synthetic ID to TaskList objects so that every task list can be referenced by a single, immutable identifier that survives renames and moves. This is the second phase of stable ID adoption, following the pattern established by `note-stable-ids`.

## Glossary

- **TaskList**: A markdown file on disk following the `tasks_<title>.md` naming convention, represented by the `TaskList` protobuf message.
- **TaskListService**: The gRPC/Connect service defined in `tasks.proto` that exposes CRUD operations for task lists.
- **TaskList_ID**: A stable, opaque, synthetic identifier assigned to a TaskList at creation time. The TaskList_ID remains constant for the lifetime of the TaskList regardless of renames or moves.
- **File_Path**: The relative filesystem path of a task list file within the data directory (e.g. `tasks_Groceries.md` or `subfolder/tasks_Groceries.md`).
- **ID_Registry**: A persistent mapping from TaskList_ID to the current File_Path of each task list, stored on disk alongside the data directory as `.tasklist_id_registry.json`.
- **CreateTaskList_RPC**: The RPC that creates a new task list file and returns the resulting TaskList message.
- **GetTaskList_RPC**: The RPC that retrieves a single task list by identifier.
- **ListTaskLists_RPC**: The RPC that returns all task lists under a given parent directory.
- **UpdateTaskList_RPC**: The RPC that overwrites the tasks of an existing task list.
- **DeleteTaskList_RPC**: The RPC that removes a task list file from disk.

## Requirements

### Requirement 1: TaskList_ID Generation
**User Story:** As a client developer, I want every task list to have a unique synthetic ID assigned at creation time, so that I can reference task lists without depending on mutable path or title fields.
#### Acceptance Criteria
1. WHEN a task list is created via CreateTaskList_RPC, THE TaskListService SHALL generate a TaskList_ID and include the TaskList_ID in the returned TaskList message.
2. THE TaskListService SHALL generate each TaskList_ID as a version-4 UUID formatted as a lowercase hyphenated string.
3. THE TaskListService SHALL ensure that no two task lists share the same TaskList_ID within a single data directory.
4. THE TaskList_ID SHALL remain constant for the lifetime of the TaskList, regardless of changes to the task list title, tasks, or parent directory.

### Requirement 2: ID_Registry Persistence
**User Story:** As a backend operator, I want the mapping from TaskList_ID to File_Path to be persisted on disk, so that IDs survive server restarts.
#### Acceptance Criteria
1. WHEN a task list is created via CreateTaskList_RPC, THE TaskListService SHALL persist the TaskList_ID-to-File_Path mapping in the ID_Registry before returning the response.
2. WHEN a task list is deleted via DeleteTaskList_RPC, THE TaskListService SHALL remove the corresponding TaskList_ID entry from the ID_Registry.
3. IF the ID_Registry file is missing or empty on server startup, THEN THE TaskListService SHALL treat the registry as empty and assign new TaskList_IDs to subsequently created task lists.
4. THE TaskListService SHALL write the ID_Registry atomically so that a crash during write does not corrupt the existing registry data.
5. THE TaskListService SHALL store the ID_Registry as a JSON file named `.tasklist_id_registry.json` at a well-known path relative to the data directory.

### Requirement 3: Protobuf Schema Changes
**User Story:** As a client developer, I want the TaskList protobuf message and request messages to include an `id` field, so that I can use stable IDs in all API interactions.
#### Acceptance Criteria
1. THE TaskList protobuf message SHALL include a string field named `id` that carries the TaskList_ID.
2. THE GetTaskListRequest message SHALL use a string field named `id` as the sole lookup key.
3. THE UpdateTaskListRequest message SHALL use a string field named `id` as the sole lookup key.
4. THE DeleteTaskListRequest message SHALL use a string field named `id` as the sole lookup key.
5. THE GetTaskListRequest, UpdateTaskListRequest, and DeleteTaskListRequest messages SHALL remove the `file_path` field since no backward compatibility is required (pre-release).

### Requirement 4: GetTaskList_RPC by ID
**User Story:** As a client developer, I want to retrieve a task list by its stable ID, so that my lookups remain valid even after the task list is renamed or moved.
#### Acceptance Criteria
1. WHEN a GetTaskListRequest contains a non-empty `id` field, THE GetTaskList_RPC SHALL resolve the TaskList_ID to a File_Path via the ID_Registry and return the corresponding TaskList.
2. WHEN a GetTaskListRequest contains an `id` that does not exist in the ID_Registry, THE GetTaskList_RPC SHALL return a NotFound error.

### Requirement 5: UpdateTaskList_RPC by ID
**User Story:** As a client developer, I want to update a task list by its stable ID, so that task updates are not affected by concurrent renames.
#### Acceptance Criteria
1. WHEN an UpdateTaskListRequest contains a non-empty `id` field, THE UpdateTaskList_RPC SHALL resolve the TaskList_ID to a File_Path via the ID_Registry and update the corresponding task list file.
2. WHEN an UpdateTaskListRequest contains an `id` that does not exist in the ID_Registry, THE UpdateTaskList_RPC SHALL return a NotFound error.

### Requirement 6: DeleteTaskList_RPC by ID
**User Story:** As a client developer, I want to delete a task list by its stable ID, so that deletion targets the correct task list even if the file was renamed.
#### Acceptance Criteria
1. WHEN a DeleteTaskListRequest contains a non-empty `id` field, THE DeleteTaskList_RPC SHALL resolve the TaskList_ID to a File_Path via the ID_Registry, delete the task list file, and remove the entry from the ID_Registry.
2. WHEN a DeleteTaskListRequest contains an `id` that does not exist in the ID_Registry, THE DeleteTaskList_RPC SHALL return a NotFound error.

### Requirement 7: ListTaskLists_RPC Includes IDs
**User Story:** As a client developer, I want listed task lists to include their stable IDs, so that I can use the ID for subsequent operations without an extra lookup.
#### Acceptance Criteria
1. WHEN task lists are listed via ListTaskLists_RPC, THE TaskListService SHALL include the TaskList_ID in each returned TaskList message.
2. WHEN a task list file exists on disk but has no corresponding entry in the ID_Registry, THE ListTaskLists_RPC SHALL return the task list with an empty `id` field rather than omitting the task list or failing.

### Requirement 8: CreateTaskList_RPC Response Includes ID
**User Story:** As a client developer, I want the CreateTaskList response to include the newly assigned ID, so that I can immediately use the stable ID for subsequent operations.
#### Acceptance Criteria
1. WHEN a task list is successfully created, THE CreateTaskList_RPC SHALL return a TaskList message where the `id` field contains the newly generated TaskList_ID.
2. WHEN a task list is successfully created, THE CreateTaskList_RPC SHALL persist the ID_Registry entry before returning the response, so that subsequent GetTaskList_RPC calls by ID succeed immediately.

### Requirement 9: ID Validation
**User Story:** As a backend developer, I want incoming IDs to be validated, so that malformed identifiers are rejected early with clear error messages.
#### Acceptance Criteria
1. WHEN a request contains a non-empty `id` field that is not a valid version-4 UUID, THE TaskListService SHALL return an InvalidArgument error.
2. THE TaskListService SHALL validate the `id` field before performing any file system operations.

### Requirement 10: Round-Trip Property
**User Story:** As a developer, I want a round-trip guarantee that creating a task list and then retrieving it by ID produces an equivalent TaskList, so that the ID-based path is trustworthy.
#### Acceptance Criteria
1. FOR ALL valid title and tasks values, creating a task list via CreateTaskList_RPC and then retrieving the task list via GetTaskList_RPC using the returned TaskList_ID SHALL produce a TaskList with the same `id`, `title`, `tasks`, and `file_path` fields.
2. FOR ALL valid title and tasks values, creating a task list via CreateTaskList_RPC and then listing task lists via ListTaskLists_RPC SHALL include a TaskList whose `id` matches the one returned by CreateTaskList_RPC.
