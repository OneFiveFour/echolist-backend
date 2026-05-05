# Requirements Document

## Introduction

Add a `GetMainTask` RPC to the existing `TaskListService` that allows the client to fetch a single `MainTask` by its ID without loading the entire parent `TaskList`. This supports the client's dedicated task settings screen, where the navigation route only carries the `mainTaskId` and the ViewModel needs to load the task's current state directly.

## Glossary

- **TaskListService**: The gRPC/Connect service that manages task lists and their tasks.
- **MainTask**: A top-level task within a task list. Contains fields such as description, is_done, due_date, recurrence, and a list of sub_tasks.
- **SubTask**: A child task nested under a MainTask.
- **Client**: The mobile or web application consuming the TaskListService RPCs.
- **Authenticated_User**: The user whose identity has been verified via JWT token in the Authorization header.

## Requirements

### Requirement 1: Fetch a MainTask by ID

**User Story:** As a client developer, I want to fetch a single MainTask by its ID, so that I can populate the task settings screen without loading the entire parent TaskList.

#### Acceptance Criteria

1. WHEN a valid GetMainTaskRequest with a known main task ID is received, THE TaskListService SHALL return a GetMainTaskResponse containing the full MainTask message including its sub_tasks.
2. WHEN a GetMainTaskRequest with an ID that does not correspond to any MainTask is received, THE TaskListService SHALL return a NOT_FOUND error.
3. WHEN a GetMainTaskRequest with an invalid ID format is received, THE TaskListService SHALL return an INVALID_ARGUMENT error.

### Requirement 2: Proto Definition

**User Story:** As a client developer, I want a well-defined proto contract for GetMainTask, so that I can generate typed client code.

#### Acceptance Criteria

1. THE TaskListService SHALL define a `GetMainTask` RPC that accepts a `GetMainTaskRequest` and returns a `GetMainTaskResponse`.
2. THE GetMainTaskRequest message SHALL contain a single `string id` field identifying the MainTask to retrieve.
3. THE GetMainTaskResponse message SHALL contain a single `MainTask main_task` field with the full task data.

### Requirement 3: Authentication Enforcement

**User Story:** As a system operator, I want the GetMainTask RPC to require authentication, so that unauthenticated callers cannot access task data.

#### Acceptance Criteria

1. WHEN a GetMainTask request is received without a valid Authorization header, THE TaskListService SHALL return an UNAUTHENTICATED error.
2. WHEN a GetMainTask request is received with an expired or invalid token, THE TaskListService SHALL return an UNAUTHENTICATED error.

### Requirement 4: Response Completeness

**User Story:** As a client developer, I want the returned MainTask to include all fields and nested sub_tasks, so that the settings screen can display and edit due date, recurrence, and subtask state.

#### Acceptance Criteria

1. THE GetMainTaskResponse SHALL include the MainTask's id, description, is_done, due_date, and recurrence fields.
2. THE GetMainTaskResponse SHALL include all SubTasks belonging to the MainTask, each with id, description, and is_done fields.
3. THE SubTasks in the response SHALL be ordered by their stored position.

### Requirement 5: Database Query

**User Story:** As a backend developer, I want a dedicated database query to look up a MainTask by ID, so that the RPC handler does not need to load an entire task list.

#### Acceptance Criteria

1. WHEN the database is queried for a MainTask by ID, THE Database SHALL return the task row and its associated subtask rows without loading the parent task list's other tasks.
2. IF the queried ID does not exist in the tasks table as a main task, THEN THE Database SHALL return a not-found indicator.
