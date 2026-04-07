# Requirements Document

## Introduction

AutoDelete feature for task lists in the echolist-backend. Each TaskList gains a boolean `isAutoDelete` flag that controls what happens when tasks are marked as done via the UpdateTaskList RPC. When AutoDelete is enabled, completing a MainTask causes the backend to remove that MainTask and all its SubTasks from the list; completing a SubTask causes the backend to remove just that SubTask. When AutoDelete is disabled, the existing behavior is preserved: tasks are simply marked as done and nothing is deleted. Manual deletion of a MainTask (via the existing UpdateTaskList flow where the frontend omits the task) always cascades to its SubTasks regardless of the AutoDelete setting.

The `isAutoDelete` flag is persisted as part of the TaskList and exposed through the protobuf API on Create, Update, Get, and List operations.

## Glossary

- **Task_Service**: The Connect-Go RPC service that handles all task-related API operations (create, read, update, delete task lists)
- **Task_List**: A single `.md` file with a `tasks_` filename prefix stored in the Data_Directory tree, containing zero or more main tasks and an `isAutoDelete` flag
- **Main_Task**: A top-level task entry within a task list; can be open or done, can have subtasks
- **Sub_Task**: A child task nested under a main task; can be open or done
- **AutoDelete_Mode**: A boolean property of a Task_List that controls whether tasks marked as done are automatically removed from the list
- **Registry**: The JSON file (`.tasklist_id_registry.json`) that maps task list UUIDs to file paths
- **Task_File_Parser**: The component that reads a task file from disk and produces an in-memory task list representation
- **Task_File_Printer**: The component that serializes an in-memory task list back into the human-readable task file format

## Requirements

### Requirement 1: AutoDelete Flag on TaskList Proto Message

**User Story:** As a developer, I want the TaskList protobuf message to include an `is_auto_delete` boolean field, so that clients can read and set the AutoDelete mode for each task list.

#### Acceptance Criteria

1. THE Task_Service SHALL include a boolean `is_auto_delete` field on the `TaskList` protobuf message
2. THE Task_Service SHALL include a boolean `is_auto_delete` field on the `CreateTaskListRequest` protobuf message
3. THE Task_Service SHALL include a boolean `is_auto_delete` field on the `UpdateTaskListRequest` protobuf message
4. WHEN a GetTaskList or ListTaskLists response is returned, THE Task_Service SHALL populate the `is_auto_delete` field with the persisted value for that task list

### Requirement 2: AutoDelete Flag Persistence

**User Story:** As a user, I want the AutoDelete setting to be saved with my task list, so that the setting persists across sessions.

#### Acceptance Criteria

1. THE Task_Service SHALL treat `is_auto_delete` as a mandatory field on CreateTaskList and UpdateTaskList requests, persisting whatever value the client provides
2. WHEN a CreateTaskList request is received, THE Task_Service SHALL persist the `is_auto_delete` value exactly as provided by the client (no server-side default)
3. WHEN an UpdateTaskList request is received, THE Task_Service SHALL update the persisted AutoDelete flag to match the request value
4. THE Task_Service SHALL persist the `is_auto_delete` flag in the task list registry alongside the existing id-to-filePath mapping
5. FOR ALL task lists, reading the persisted AutoDelete flag after a create or update SHALL return the value that was last written (round-trip property)

### Requirement 3: AutoDelete Behavior for MainTask Marked as Done

**User Story:** As a user, I want completed main tasks to be automatically removed when AutoDelete is enabled, so that my task list stays clean without manual cleanup.

#### Acceptance Criteria

1. WHILE AutoDelete_Mode is enabled on a Task_List, WHEN a Main_Task is marked as done in an UpdateTaskList request, THE Task_Service SHALL remove that Main_Task and all of its Sub_Tasks from the persisted task list
2. WHILE AutoDelete_Mode is disabled on a Task_List, WHEN a Main_Task is marked as done in an UpdateTaskList request, THE Task_Service SHALL mark the Main_Task as done and retain the Main_Task and all of its Sub_Tasks in the persisted task list
3. WHILE AutoDelete_Mode is enabled on a Task_List, WHEN a recurring Main_Task is marked as done in an UpdateTaskList request, THE Task_Service SHALL advance the recurrence (reset to open with the next due date) instead of deleting the Main_Task

### Requirement 4: AutoDelete Behavior for SubTask Marked as Done

**User Story:** As a user, I want completed subtasks to be automatically removed when AutoDelete is enabled, so that I only see remaining work.

#### Acceptance Criteria

1. WHILE AutoDelete_Mode is enabled on a Task_List, WHEN a Sub_Task is marked as done in an UpdateTaskList request, THE Task_Service SHALL remove that Sub_Task from its parent Main_Task in the persisted task list
2. WHILE AutoDelete_Mode is disabled on a Task_List, WHEN a Sub_Task is marked as done in an UpdateTaskList request, THE Task_Service SHALL mark the Sub_Task as done and retain the Sub_Task in the persisted task list

### Requirement 5: Manual MainTask Deletion Cascades to SubTasks

**User Story:** As a user, I want manually deleting a main task to also remove its subtasks regardless of AutoDelete mode, so that orphaned subtasks do not remain.

#### Acceptance Criteria

1. WHEN a Main_Task present in the persisted task list is absent from the UpdateTaskList request task list, THE Task_Service SHALL treat the Main_Task as manually deleted and remove the Main_Task and all of its Sub_Tasks from the persisted task list
2. THE Task_Service SHALL apply manual Main_Task deletion regardless of the AutoDelete_Mode setting on the Task_List

### Requirement 6: AutoDelete Combined with Recurrence

**User Story:** As a user, I want recurring tasks to advance their due date instead of being deleted when AutoDelete is on, so that I do not lose my recurring schedule.

#### Acceptance Criteria

1. WHILE AutoDelete_Mode is enabled on a Task_List, WHEN a recurring Main_Task is marked as done, THE Task_Service SHALL reset the Main_Task status to open, compute the next due date from the RRULE pattern, and retain the Main_Task in the persisted task list
2. WHILE AutoDelete_Mode is enabled on a Task_List, WHEN a non-recurring Main_Task is marked as done, THE Task_Service SHALL remove the Main_Task and all of its Sub_Tasks from the persisted task list

### Requirement 7: UpdateTaskList Response Reflects AutoDelete Outcome

**User Story:** As a developer, I want the UpdateTaskList response to reflect the final state after AutoDelete processing, so that the client can update its UI without a separate read call.

#### Acceptance Criteria

1. WHEN an UpdateTaskList request is processed with AutoDelete_Mode enabled, THE Task_Service SHALL return the task list in the response with auto-deleted tasks already removed
2. WHEN an UpdateTaskList request is processed with AutoDelete_Mode disabled, THE Task_Service SHALL return the task list in the response with all tasks present (including those marked as done)
3. THE Task_Service SHALL include the current `is_auto_delete` value in the UpdateTaskList response


