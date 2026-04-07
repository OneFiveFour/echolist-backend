# Implementation Plan: Note Registry Struct Refactor

## Overview

Refactor `notes/registry.go` from `map[string]string` to `map[string]registryEntry`, update all callers in the notes package, simplify `readRegistryReverse` in `file/list_files.go`, and update all affected tests. Each task builds incrementally so the codebase compiles and passes tests at every checkpoint.

## Tasks

- [x] 1. Define registryEntry struct and update registry functions
  - [x] 1.1 Add `registryEntry` struct and update `registryRead`, `registryWrite`, `registryLookup`, `registryAdd` signatures in `notes/registry.go`
    - Define `type registryEntry struct { FilePath string \`json:"filePath"\` }`
    - Change `registryRead` return type to `(map[string]registryEntry, error)`
    - Change `registryWrite` parameter to `map[string]registryEntry`
    - Change `registryLookup` to return `(registryEntry, bool, error)`
    - Change `registryAdd` to accept `(regPath, id string, entry registryEntry)`
    - `registryRemove` signature stays the same
    - _Requirements: 1.1, 1.2, 2.1, 2.2, 2.3, 2.4, 2.5_

  - [x] 1.2 Update all callers in `notes/create_note.go`, `notes/get_note.go`, `notes/delete_note.go`, `notes/update_note.go`, and `notes/list_notes.go`
    - `CreateNote`: change `registryAdd(regPath, id, relativeFilePath)` → `registryAdd(regPath, id, registryEntry{FilePath: relativeFilePath})`
    - `GetNote`: change `filePath, found, err := registryLookup(...)` → `entry, found, err := registryLookup(...); filePath := entry.FilePath`
    - `DeleteNote`: same pattern as GetNote for the lookup result
    - `UpdateNote`: adapt both lookup and the two `registryAdd` calls (normal path and rollback) to use `registryEntry{FilePath: ...}`
    - `ListNotes`: iterate registry values using `.FilePath` for the reverse map key
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

  - [x] 1.3 Update existing unit tests in `notes/registry_test.go`
    - `TestRegistryAdd_ThenLookup`: pass `registryEntry{FilePath: "note_Meeting.md"}`, assert on `.FilePath`
    - `TestRegistryRemove`: adapt `registryAdd` call to use `registryEntry`
    - `TestRegistryRead_MissingFile`, `TestRegistryRead_EmptyFile`: verify returned map type is `map[string]registryEntry`
    - _Requirements: 5.1, 5.2, 5.3_

- [x] 2. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 3. Simplify readRegistryReverse in file/list_files.go
  - [x] 3.1 Replace `readRegistryReverse` body with single-pass `json.Unmarshal` into `map[string]struct{ FilePath string }`
    - Remove `json.RawMessage` multi-pass parsing and string-value fallback logic
    - Use a single `json.Unmarshal` call into a typed anonymous struct map
    - Return empty map on any read or parse error
    - Remove the `"encoding/json"` import is no longer needed — keep it since the new implementation still uses `json.Unmarshal`
    - _Requirements: 4.1, 4.2, 4.3_

- [x] 4. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Add property-based tests
  - [x] 5.1 Write property test for registry write-then-read round trip
    - **Property 1: Registry write-then-read round trip**
    - **Validates: Requirements 2.1, 2.2, 5.4**
    - Create `notes/registry_property_test.go`
    - Use `pgregory.net/rapid` to generate arbitrary `map[string]registryEntry` maps
    - Verify `registryWrite` then `registryRead` produces an identical map
    - Tag: `Feature: note-registry-struct-refactor, Property 1: Registry write-then-read round trip`

  - [x] 5.2 Write property test for add-then-lookup
    - **Property 2: Add-then-lookup returns the entry**
    - **Validates: Requirements 2.3, 2.4**
    - Generate random `(id, registryEntry)` pairs and optional pre-existing registry entries
    - Verify `registryAdd` followed by `registryLookup` returns the added entry with `found=true`
    - Tag: `Feature: note-registry-struct-refactor, Property 2: Add-then-lookup returns the entry`

  - [x] 5.3 Write property test for remove-then-lookup
    - **Property 3: Remove-then-lookup returns not found**
    - **Validates: Requirements 2.5**
    - Generate a registry with at least one entry, remove a known id, verify lookup returns `found=false`
    - Tag: `Feature: note-registry-struct-refactor, Property 3: Remove-then-lookup returns not found`

  - [x] 5.4 Write property test for readRegistryReverse inverse
    - **Property 4: readRegistryReverse produces correct inverse map**
    - **Validates: Requirements 4.1, 5.5**
    - Create `file/list_files_registry_property_test.go`
    - Generate a `map[string]registryEntry`, write it with `notes.registryWrite` (or write JSON directly), call `readRegistryReverse`, verify every `(filePath, id)` pair matches the original
    - Tag: `Feature: note-registry-struct-refactor, Property 4: readRegistryReverse produces correct inverse map`

- [x] 6. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests use `pgregory.net/rapid` which is already a project dependency
- No backward compatibility handling needed — pre-release software
