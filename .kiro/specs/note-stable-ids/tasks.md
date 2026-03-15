# Implementation Plan: Note Stable IDs

## Overview

Add a stable UUIDv4 identifier to every note, persisted in an on-disk JSON registry. Switch Get, Update, and Delete RPCs from `file_path`-based lookup to `id`-based lookup. Since this is pre-release, remove `file_path` from request messages entirely. Implementation language is Go, using `crypto/rand` for UUID generation, `pgregory.net/rapid` for property tests, and the existing `common.File` for atomic writes.

## Tasks

- [x] 1. Update protobuf schema and regenerate Go code
  - [x] 1.1 Modify `proto/notes/v1/notes.proto`
    - Add `string id = 1` to the `Note` message, renumber existing fields (`file_path = 2`, `title = 3`, `content = 4`, `updated_at = 5`)
    - Replace `string file_path = 1` with `string id = 1` in `GetNoteRequest`, `UpdateNoteRequest`, and `DeleteNoteRequest`
    - Keep `UpdateNoteRequest.content` as field 2
    - `CreateNoteRequest` and `ListNotesRequest` are unchanged
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

  - [x] 1.2 Regenerate Go protobuf/Connect code
    - Run `buf generate` from the `proto/` directory
    - Verify the generated code compiles
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

- [x] 2. Implement UUID validation utility (`notes/uuid.go`)
  - [x] 2.1 Create `notes/uuid.go` with `validateUuidV4` function
    - Validate lowercase hyphenated UUIDv4 format: `[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`
    - Return a `connect.NewError(connect.CodeInvalidArgument, ...)` on invalid input
    - Use regex or manual parse, no external dependency
    - _Requirements: 9.1, 9.2_

  - [x] 2.2 Write property test: invalid UUID returns InvalidArgument
    - **Property 8: Invalid UUID returns InvalidArgument**
    - Create `notes/uuid_property_test.go`
    - Add `invalidUuidGen()` generator producing strings that are NOT valid UUIDv4
    - For any invalid UUID string, `validateUuidV4` returns a Connect `InvalidArgument` error
    - **Validates: Requirements 9.1**

  - [x] 2.3 Write unit tests for `validateUuidV4`
    - Create `notes/uuid_test.go`
    - Test valid UUIDv4 strings pass, uppercase rejected, wrong version rejected, empty string rejected
    - _Requirements: 9.1_

- [x] 3. Implement ID registry (`notes/registry.go`)
  - [x] 3.1 Create `notes/registry.go` with registry functions
    - Implement `registryPath(dataDir string) string` returning `<dataDir>/.note_id_registry.json`
    - Implement `registryRead(path string) (map[string]string, error)` — returns empty map if file missing or empty
    - Implement `registryWrite(path string, m map[string]string) error` — atomic write via `common.File`
    - Implement `registryLookup(regPath, id string) (string, bool, error)`
    - Implement `registryAdd(regPath, id, filePath string) error`
    - Implement `registryRemove(regPath, id string) error`
    - _Requirements: 2.1, 2.2, 2.4, 2.5_

  - [x] 3.2 Write unit tests for registry functions
    - Create `notes/registry_test.go`
    - Test `registryRead` with missing file returns empty map (edge case from Req 2.3)
    - Test `registryRead` with empty file returns empty map (edge case from Req 2.3)
    - Test `registryAdd` then `registryLookup` round trip
    - Test `registryRemove` removes entry
    - _Requirements: 2.1, 2.2, 2.3, 2.5_

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 5. Update CreateNote RPC to generate and persist IDs
  - [ ] 5.1 Modify `notes/create_note.go`
    - After writing the note file, generate a UUIDv4 via `crypto/rand` (16 random bytes, set version and variant bits, format as lowercase hyphenated string)
    - Acquire registry lock via `s.locks.Lock(registryPath(s.dataDir))`, call `registryAdd` to persist the id→filePath mapping
    - Populate `note.Id` in the response
    - _Requirements: 1.1, 1.2, 1.3, 2.1, 8.1, 8.2_

  - [ ] 5.2 Write property test: created ID is valid UUIDv4
    - **Property 2: Created ID is valid UUIDv4**
    - Add to `notes/create_note_property_test.go`
    - For any valid title and content, the `id` field returned by CreateNote matches the UUIDv4 pattern
    - **Validates: Requirements 1.2**

  - [ ] 5.3 Write property test: all created IDs are unique
    - **Property 3: All created IDs are unique**
    - Add to `notes/create_note_property_test.go`
    - For any sequence of N CreateNote calls with distinct titles, all returned `id` values are pairwise distinct
    - **Validates: Requirements 1.3**

- [ ] 6. Update GetNote RPC to resolve by ID
  - [ ] 6.1 Modify `notes/get_note.go`
    - Validate the `id` field with `validateUuidV4` before any filesystem operations
    - Call `registryLookup` to resolve `id` to a `filePath`
    - Return `NotFound` if ID not in registry or if resolved file doesn't exist on disk
    - Use the resolved `filePath` for existing file-read logic
    - Populate `note.Id` in the response
    - _Requirements: 4.1, 4.2, 9.1, 9.2_

- [ ] 7. Update UpdateNote RPC to resolve by ID
  - [ ] 7.1 Modify `notes/update_note.go`
    - Validate the `id` field with `validateUuidV4`
    - Call `registryLookup` to resolve `id` to a `filePath`
    - Return `NotFound` if ID not in registry
    - Use the resolved `filePath` for existing update logic
    - Populate `note.Id` in the response
    - _Requirements: 5.1, 5.2, 9.1, 9.2_

  - [ ] 7.2 Write property test: update by ID preserves the Note_ID
    - **Property 4: Update by ID preserves the Note_ID**
    - Add to `notes/update_note_property_test.go`
    - For any created note and new content, UpdateNote with the note's `id` returns a Note with the same `id` and the new content
    - **Validates: Requirements 1.4, 5.1**

- [ ] 8. Update DeleteNote RPC to resolve by ID and clean up registry
  - [ ] 8.1 Modify `notes/delete_note.go`
    - Validate the `id` field with `validateUuidV4`
    - Acquire registry lock, call `registryLookup` to resolve `id`
    - Acquire note file lock, delete the file, call `registryRemove`, release locks
    - Return `NotFound` if ID not in registry
    - _Requirements: 6.1, 6.2, 2.2, 9.1, 9.2_

  - [ ] 8.2 Write property test: delete by ID removes file and registry entry
    - **Property 5: Delete by ID removes file and registry entry**
    - Add to `notes/delete_note_property_test.go` (new file)
    - For any created note, DeleteNote with its `id` succeeds, and subsequent GetNote returns NotFound
    - **Validates: Requirements 2.2, 6.1**

- [ ] 9. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 10. Update ListNotes RPC to include IDs
  - [ ] 10.1 Modify `notes/list_notes.go`
    - After building the notes list from the filesystem, read the registry once via `registryRead`
    - Build a reverse map (filePath → id) and attach IDs to each note
    - Notes without a registry entry get an empty `id` field (do not omit or fail)
    - _Requirements: 7.1, 7.2_

  - [ ] 10.2 Write property test: create then list includes the created note's ID
    - **Property 7: Create then list includes the created note's ID**
    - Add to `notes/list_notes_property_test.go`
    - For any valid title and content, CreateNote then ListNotes returns a list containing a Note whose `id` matches the one from CreateNote
    - **Validates: Requirements 7.1, 10.2**

  - [ ] 10.3 Write unit test: orphan note file returns empty ID
    - Add to `notes/list_notes_test.go`
    - Create a note file on disk without a registry entry, call ListNotes, verify the note is returned with an empty `id` field
    - _Requirements: 7.2_

- [ ] 11. Update existing tests and add round-trip property test
  - [ ] 11.1 Update existing test files to use new protobuf fields
    - Update `notes/note_round_trip_property_test.go` — GetNote now uses `Id` instead of `FilePath`, UpdateNote uses `Id`
    - Update `notes/create_note_property_test.go` — existing property tests still compile with new proto fields
    - Update `notes/not_found_property_test.go`, `notes/path_traversal_property_test.go`, `notes/update_note_property_test.go`, `notes/list_notes_property_test.go` — adapt to new request message shapes
    - Update `notes/create_note_test.go`, `notes/get_note_test.go`, `notes/update_note_test.go`, `notes/delete_note_test.go`, `notes/list_notes_test.go`, `notes/content_limit_test.go` — adapt to new request/response fields
    - _Requirements: 3.2, 3.3, 3.4, 3.5_

  - [ ] 11.2 Write property test: create-then-get round trip with ID
    - **Property 1: Create-then-get round trip**
    - Update `notes/note_round_trip_property_test.go`
    - For any valid title and content, CreateNote then GetNote by `id` produces a Note with the same `id`, `title`, `content`, and `file_path`
    - **Validates: Requirements 1.1, 2.1, 4.1, 8.1, 8.2, 10.1**

  - [ ] 11.3 Write property test: non-existent ID returns NotFound
    - **Property 6: Non-existent ID returns NotFound**
    - Add to `notes/not_found_property_test.go`
    - Add `uuidV4Gen()` generator producing valid UUIDv4 strings
    - For any valid UUIDv4 never used in CreateNote, GetNote/UpdateNote/DeleteNote return NotFound
    - **Validates: Requirements 4.2, 5.2, 6.2**

  - [ ] 11.4 Write property test: invalid UUID rejected by Get/Update/Delete RPCs
    - **Property 8: Invalid UUID returns InvalidArgument (RPC level)**
    - Add to `notes/not_found_property_test.go` or a new file
    - For any invalid UUID string, GetNote/UpdateNote/DeleteNote return InvalidArgument
    - **Validates: Requirements 9.1, 9.2**

- [ ] 12. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks including tests are mandatory
- The design uses Go throughout, so all code examples target Go
- Use `Id` not `ID` in Go identifiers per project convention (e.g., `note.Id`, `validateUuidV4()`)
- `id` is field number 1 in all protobuf messages — no backward compatibility needed
- `id` is the sole lookup key for Get, Update, Delete — no `file_path` fallback
- Property tests use `pgregory.net/rapid` following existing patterns in `notes/*_property_test.go`
- Registry atomic writes use `common.File` (temp file + rename)
- Locking uses existing `common.Locker` with registry file path as lock key
