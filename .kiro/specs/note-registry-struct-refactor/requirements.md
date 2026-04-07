# Requirements Document

## Introduction

The notes registry (`notes/registry.go`) currently stores entries as `map[string]string` (UUID → file path), while the task list registry (`tasks/registry.go`) uses `map[string]registryEntry` where `registryEntry` is a struct with `FilePath` and `IsAutoDelete` fields. This inconsistency caused a bug in `file/list_files.go` where `readRegistryReverse` had to handle both formats with fallback logic.

This refactor aligns the notes registry to use the same struct-based format as the task list registry, enabling a uniform JSON shape across all registries, simplifying cross-registry consumers like `readRegistryReverse`, and preparing the notes registry for future metadata fields.

## Glossary

- **Notes_Registry**: The JSON file (`.note_id_registry.json`) that maps note UUIDs to their metadata. Located in the data directory root.
- **Registry_Entry**: A struct containing metadata for a single registry item. For notes, initially contains only `FilePath string`.
- **Task_List_Registry**: The JSON file (`.tasklist_id_registry.json`) that maps task list UUIDs to `registryEntry` structs containing `FilePath` and `IsAutoDelete`.
- **readRegistryReverse**: A function in `file/list_files.go` that reads a registry file and returns the inverse map (filePath → UUID).
- **Callers**: Functions in the `notes` package that invoke `registryRead`, `registryWrite`, `registryLookup`, `registryAdd`, or `registryRemove`.
- **Struct_Format**: The notes registry JSON shape where values are objects (`{"uuid": {"filePath": "note_MyNote.md"}}`).

## Requirements

### Requirement 1: Define a registryEntry struct in the notes package

**User Story:** As a developer, I want the notes registry to use a typed struct for its values, so that the data model is extensible and consistent with the task list registry.

#### Acceptance Criteria

1. THE Notes_Registry module SHALL define a `registryEntry` struct with a `FilePath string` field tagged as `json:"filePath"`.
2. THE Notes_Registry module SHALL use `map[string]registryEntry` as the internal registry type instead of `map[string]string`.

### Requirement 2: Update registry functions to use registryEntry

**User Story:** As a developer, I want all notes registry functions to accept and return `registryEntry` structs, so that callers work with the new typed format.

#### Acceptance Criteria

1. WHEN `registryRead` is called, THE Notes_Registry SHALL return `map[string]registryEntry` instead of `map[string]string`.
2. WHEN `registryWrite` is called, THE Notes_Registry SHALL accept `map[string]registryEntry` and serialize each entry as a JSON object with a `filePath` field.
3. WHEN `registryLookup` is called, THE Notes_Registry SHALL return a `registryEntry` value and a boolean indicating whether the entry was found.
4. WHEN `registryAdd` is called, THE Notes_Registry SHALL accept an `id` string and a `registryEntry` value instead of an `id` string and a plain file path string.
5. WHEN `registryRemove` is called, THE Notes_Registry SHALL continue to accept an `id` string and remove the corresponding entry.

### Requirement 3: Update all callers in the notes package

**User Story:** As a developer, I want all note operations (create, get, update, delete, list) to work with the new struct-based registry, so that the refactor does not break any existing functionality.

#### Acceptance Criteria

1. WHEN `CreateNote` adds a registry entry, THE Notes_Registry caller SHALL pass a `registryEntry{FilePath: relativeFilePath}` instead of a plain string.
2. WHEN `GetNote` looks up a registry entry, THE Notes_Registry caller SHALL extract the `FilePath` field from the returned `registryEntry`.
3. WHEN `DeleteNote` looks up a registry entry, THE Notes_Registry caller SHALL extract the `FilePath` field from the returned `registryEntry`.
4. WHEN `UpdateNote` looks up or updates a registry entry, THE Notes_Registry caller SHALL use `registryEntry` for both lookup results and add operations.
5. WHEN `ListNotes` reads the full registry and builds a reverse map, THE Notes_Registry caller SHALL iterate over `registryEntry` values and use the `FilePath` field as the map key.

### Requirement 4: Simplify readRegistryReverse in file/list_files.go

**User Story:** As a developer, I want `readRegistryReverse` to use a single parsing strategy for both registries, so that the dual-format fallback logic is removed.

#### Acceptance Criteria

1. THE readRegistryReverse function SHALL unmarshal registry files directly into a `map[string]struct{ FilePath string }` (or equivalent typed map) using a single `json.Unmarshal` call.
2. THE readRegistryReverse function SHALL NOT use `json.RawMessage`, multi-pass parsing, or fallback logic for different value formats.
3. IF a registry file cannot be parsed, THEN THE readRegistryReverse function SHALL return an empty map.

### Requirement 5: Update all affected tests

**User Story:** As a developer, I want all tests to reflect the new struct-based registry format, so that the test suite validates the refactored behavior.

#### Acceptance Criteria

1. WHEN notes registry tests call `registryAdd`, THE test code SHALL pass a `registryEntry` struct instead of a plain string.
2. WHEN notes registry tests call `registryLookup`, THE test code SHALL assert on the `FilePath` field of the returned `registryEntry`.
3. WHEN notes registry tests validate JSON on disk, THE test code SHALL expect the Struct_Format (`{"uuid": {"filePath": "..."}}`).
4. THE test suite SHALL include a round-trip property test that verifies: for any valid `map[string]registryEntry`, writing it with `registryWrite` and reading it back with `registryRead` produces an identical map.
5. THE test suite SHALL include a test that verifies `readRegistryReverse` works with the unified Struct_Format for both registries.
