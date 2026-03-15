# Requirements Document

## Introduction

Notes in echolist-backend currently use a composite natural key of `parent_dir + title` (materialized as `file_path`). This works for simple lookups but causes friction across layers: cache invalidation on rename requires atomic updates to every reference, navigation state becomes stale when a note is renamed or moved, conflict resolution for same-titled notes created on different devices is ambiguous, and passing a two-part key through ViewModel → Repository → Cache → Network adds coupling.

This feature adds a stable synthetic ID to Note objects so that every note can be referenced by a single, immutable identifier that survives renames and moves. This is the first of two phases — notes first, task lists later.

## Glossary

- **Note**: A markdown file on disk following the `note_<title>.md` naming convention, represented by the `Note` protobuf message.
- **NoteService**: The gRPC/Connect service defined in `notes.proto` that exposes CRUD operations for notes.
- **Note_ID**: A stable, opaque, synthetic identifier assigned to a Note at creation time. The Note_ID remains constant for the lifetime of the Note regardless of renames or moves.
- **File_Path**: The relative filesystem path of a note file within the data directory (e.g. `note_Meeting.md` or `subfolder/note_Meeting.md`).
- **ID_Registry**: A persistent mapping from Note_ID to the current File_Path of each note, stored on disk alongside the data directory.
- **CreateNote_RPC**: The RPC that creates a new note file and returns the resulting Note message.
- **GetNote_RPC**: The RPC that retrieves a single note by identifier.
- **ListNotes_RPC**: The RPC that returns all notes under a given parent directory.
- **UpdateNote_RPC**: The RPC that overwrites the content of an existing note.
- **DeleteNote_RPC**: The RPC that removes a note file from disk.

## Requirements

### Requirement 1: Note_ID Generation

**User Story:** As a client developer, I want every note to have a unique synthetic ID assigned at creation time, so that I can reference notes without depending on mutable path or title fields.

#### Acceptance Criteria

1. WHEN a note is created via CreateNote_RPC, THE NoteService SHALL generate a Note_ID and include the Note_ID in the returned Note message.
2. THE NoteService SHALL generate each Note_ID as a version-4 UUID formatted as a lowercase hyphenated string (e.g. `550e8400-e29b-41d4-a716-446655440000`).
3. THE NoteService SHALL ensure that no two notes share the same Note_ID within a single data directory.
4. THE Note_ID SHALL remain constant for the lifetime of the Note, regardless of changes to the note title, content, or parent directory.

### Requirement 2: ID_Registry Persistence

**User Story:** As a backend operator, I want the mapping from Note_ID to File_Path to be persisted on disk, so that IDs survive server restarts.

#### Acceptance Criteria

1. WHEN a note is created via CreateNote_RPC, THE NoteService SHALL persist the Note_ID-to-File_Path mapping in the ID_Registry before returning the response.
2. WHEN a note is deleted via DeleteNote_RPC, THE NoteService SHALL remove the corresponding Note_ID entry from the ID_Registry.
3. IF the ID_Registry file is missing or empty on server startup, THEN THE NoteService SHALL treat the registry as empty and assign new Note_IDs to subsequently created notes.
4. THE NoteService SHALL write the ID_Registry atomically so that a crash during write does not corrupt the existing registry data.
5. THE NoteService SHALL store the ID_Registry as a JSON file at a well-known path relative to the data directory.

### Requirement 3: Protobuf Schema Changes

**User Story:** As a client developer, I want the Note protobuf message and request messages to include an `id` field, so that I can use stable IDs in all API interactions.

#### Acceptance Criteria

1. THE Note protobuf message SHALL include a string field named `id` that carries the Note_ID.
2. THE GetNoteRequest message SHALL use a string field named `id` as the sole lookup key.
3. THE UpdateNoteRequest message SHALL use a string field named `id` as the sole lookup key.
4. THE DeleteNoteRequest message SHALL use a string field named `id` as the sole lookup key.
5. THE GetNoteRequest, UpdateNoteRequest, and DeleteNoteRequest messages SHALL remove the `file_path` field since no backward compatibility is required (pre-release).

### Requirement 4: GetNote_RPC by ID

**User Story:** As a client developer, I want to retrieve a note by its stable ID, so that my lookups remain valid even after the note is renamed or moved.

#### Acceptance Criteria

1. WHEN a GetNoteRequest contains a non-empty `id` field, THE GetNote_RPC SHALL resolve the Note_ID to a File_Path via the ID_Registry and return the corresponding Note.
2. WHEN a GetNoteRequest contains an `id` that does not exist in the ID_Registry, THE GetNote_RPC SHALL return a NotFound error.

### Requirement 5: UpdateNote_RPC by ID

**User Story:** As a client developer, I want to update a note by its stable ID, so that content updates are not affected by concurrent renames.

#### Acceptance Criteria

1. WHEN an UpdateNoteRequest contains a non-empty `id` field, THE UpdateNote_RPC SHALL resolve the Note_ID to a File_Path via the ID_Registry and update the corresponding note file.
2. WHEN an UpdateNoteRequest contains an `id` that does not exist in the ID_Registry, THE UpdateNote_RPC SHALL return a NotFound error.

### Requirement 6: DeleteNote_RPC by ID

**User Story:** As a client developer, I want to delete a note by its stable ID, so that deletion targets the correct note even if the file was renamed.

#### Acceptance Criteria

1. WHEN a DeleteNoteRequest contains a non-empty `id` field, THE DeleteNote_RPC SHALL resolve the Note_ID to a File_Path via the ID_Registry, delete the note file, and remove the entry from the ID_Registry.
2. WHEN a DeleteNoteRequest contains an `id` that does not exist in the ID_Registry, THE DeleteNote_RPC SHALL return a NotFound error.

### Requirement 7: ListNotes_RPC Includes IDs

**User Story:** As a client developer, I want listed notes to include their stable IDs, so that I can use the ID for subsequent operations without an extra lookup.

#### Acceptance Criteria

1. WHEN notes are listed via ListNotes_RPC, THE NoteService SHALL include the Note_ID in each returned Note message.
2. WHEN a note file exists on disk but has no corresponding entry in the ID_Registry, THE ListNotes_RPC SHALL return the note with an empty `id` field rather than omitting the note or failing.

### Requirement 8: CreateNote_RPC Response Includes ID

**User Story:** As a client developer, I want the CreateNote response to include the newly assigned ID, so that I can immediately use the stable ID for subsequent operations.

#### Acceptance Criteria

1. WHEN a note is successfully created, THE CreateNote_RPC SHALL return a Note message where the `id` field contains the newly generated Note_ID.
2. WHEN a note is successfully created, THE CreateNote_RPC SHALL persist the ID_Registry entry before returning the response, so that subsequent GetNote_RPC calls by ID succeed immediately.

### Requirement 9: ID Validation

**User Story:** As a backend developer, I want incoming IDs to be validated, so that malformed identifiers are rejected early with clear error messages.

#### Acceptance Criteria

1. WHEN a request contains a non-empty `id` field that is not a valid version-4 UUID, THE NoteService SHALL return an InvalidArgument error.
2. THE NoteService SHALL validate the `id` field before performing any file system operations.

### Requirement 10: Round-Trip Property

**User Story:** As a developer, I want a round-trip guarantee that creating a note and then retrieving it by ID produces an equivalent Note, so that the ID-based path is trustworthy.

#### Acceptance Criteria

1. FOR ALL valid title and content values, creating a note via CreateNote_RPC and then retrieving the note via GetNote_RPC using the returned Note_ID SHALL produce a Note with the same `id`, `title`, `content`, and `file_path` fields.
2. FOR ALL valid title and content values, creating a note via CreateNote_RPC and then listing notes via ListNotes_RPC SHALL include a Note whose `id` matches the one returned by CreateNote_RPC.
