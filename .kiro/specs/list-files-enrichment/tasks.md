# Implementation Plan: List Files Enrichment

## Overview

This implementation adds rich metadata to the ListFiles RPC by introducing new proto messages (`FileEntry`, `FolderMetadata`, `NoteMetadata`, `TaskListMetadata`) and refactoring the service layer to enrich each entry with type-specific metadata. The enrichment is best-effort per entry, with I/O errors logged but not propagated. Implementation follows a bottom-up approach: proto changes first, then common utilities, then service layer refactoring, and finally comprehensive testing.

## Tasks

- [x] 1. Update proto definitions and generate Go code
  - Add `ItemType` enum with values: `UNSPECIFIED`, `FOLDER`, `NOTE`, `TASK_LIST`
  - Add `FolderMetadata`, `NoteMetadata`, `TaskListMetadata` messages
  - Add `FileEntry` message with `oneof metadata` field
  - Change `ListFilesResponse.entries` from `repeated string` to `repeated FileEntry`
  - Run `buf generate` in proto/ directory to generate Go code
  - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 3.1, 3.2, 4.1, 4.2, 4.3, 5.1, 5.2, 5.3_

- [ ] 2. Export MatchesFileType function in common package
  - [ ] 2.1 Add exported MatchesFileType function to common/pathutil.go
    - Function signature: `MatchesFileType(name string, ft FileType) bool`
    - Implementation checks prefix, suffix, and minimum length
    - _Requirements: 2.2, 2.3, 2.4_
  
  - [ ] 2.2 Write unit tests for MatchesFileType
    - Test valid note filenames (note_*.md)
    - Test valid task list filenames (tasks_*.md)
    - Test invalid filenames (missing prefix, wrong suffix, too short)
    - _Requirements: 2.2, 2.3, 2.4_

- [ ] 3. Implement helper functions in file/list_files.go
  - [ ] 3.1 Implement entryPath helper function
    - Returns `name` when `requestParentDir` is empty
    - Returns `requestParentDir + "/" + name` otherwise
    - _Requirements: 6.1, 6.2_
  
  - [ ] 3.2 Implement buildFolderEntry function
    - Read subdirectory with os.ReadDir
    - Count recognized children (folders + notes + task lists) using common.MatchesFileType
    - Return FileEntry with FOLDER type and FolderMetadata
    - Log warning and set child_count=0 on ReadDir error
    - _Requirements: 2.2, 3.1, 3.2, 3.3_
  
  - [ ] 3.3 Implement buildNoteEntry function
    - Stat file for updated_at (ModTime in Unix milliseconds)
    - Read file content with os.ReadFile
    - Truncate to first 100 characters (rune-safe, not byte-safe)
    - Extract title using common.ExtractTitle
    - Return FileEntry with NOTE type and NoteMetadata
    - Log warning and use zero/empty values on I/O errors
    - _Requirements: 2.3, 4.1, 4.2, 4.3, 4.4_
  
  - [ ] 3.4 Implement buildTaskListEntry function
    - Stat file for updated_at (ModTime in Unix milliseconds)
    - Read and parse file with tasks.ParseTaskFile
    - Count total MainTask items and done MainTask items (Done == true)
    - Extract title using common.ExtractTitle
    - Return FileEntry with TASK_LIST type and TaskListMetadata
    - Log warning and use zero values on I/O or parse errors
    - _Requirements: 2.4, 5.1, 5.2, 5.3, 5.4_
  
  - [ ]* 3.5 Write unit tests for helper functions
    - Test entryPath with empty and non-empty parent_dir
    - Test buildFolderEntry with various child combinations
    - Test buildNoteEntry with different content lengths and UTF-8
    - Test buildTaskListEntry with various task counts
    - Test error handling (I/O failures, parse errors)
    - _Requirements: 3.1, 3.2, 3.3, 4.1, 4.2, 4.3, 4.4, 5.1, 5.2, 5.3, 5.4, 6.1, 6.2_

- [ ] 4. Refactor ListFiles method to use new FileEntry structure
  - [ ] 4.1 Update ListFiles to build FileEntry messages
    - Remove old string-based entry building
    - Classify each entry using IsDir() and common.MatchesFileType
    - Call appropriate builder function (buildFolderEntry, buildNoteEntry, buildTaskListEntry)
    - Skip unrecognized files
    - Remove unexported matchesFileType function (replaced by common.MatchesFileType)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 6.1, 6.2, 9.1, 9.2_
  
  - [ ]* 4.2 Write unit tests for ListFiles refactored logic
    - Test empty directory returns empty entries
    - Test directory with only unrecognized files returns empty entries
    - Test mixed directory returns correct FileEntry types
    - Test path traversal returns InvalidArgument (regression)
    - Test non-existent directory returns NotFound (regression)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 6.1, 6.2, 9.1, 9.2_

- [ ] 5. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

- [ ]* 6. Implement property-based tests for correctness properties
  - [ ]* 6.1 Write property test for classification and filtering
    - **Property 1: Classification and filtering correctness**
    - **Validates: Requirements 2.2, 2.3, 2.4, 2.5**
    - Generate random directory with mixed entries
    - Verify entry count and item_type values
    - Verify no unrecognized files in response
  
  - [ ]* 6.2 Write property test for path construction
    - **Property 2: Path construction correctness**
    - **Validates: Requirements 6.1, 6.2**
    - Generate random parent_dir values (empty and non-empty)
    - Verify path format matches expected pattern
  
  - [ ]* 6.3 Write property test for folder metadata
    - **Property 3: Folder metadata correctness**
    - **Validates: Requirements 3.1, 3.2, 3.3**
    - Generate random subdirectories with mixed children
    - Verify title equals directory name
    - Verify child_count matches independent count of recognized items
  
  - [ ]* 6.4 Write property test for note metadata
    - **Property 4: Note metadata correctness**
    - **Validates: Requirements 4.1, 4.2, 4.3, 4.4**
    - Generate random note files with varying content lengths
    - Verify title extraction, updated_at, and preview truncation
  
  - [ ]* 6.5 Write property test for task list metadata
    - **Property 5: Task list metadata correctness**
    - **Validates: Requirements 5.1, 5.2, 5.3, 5.4**
    - Generate random task files with varying task counts
    - Verify title extraction, updated_at, total_task_count, done_task_count
  
  - [ ]* 6.6 Write property test for path round-trip usability
    - **Property 6: Path round-trip usability**
    - **Validates: Requirements 6.3**
    - Generate random directory structure
    - Verify returned paths work in GetNote/GetTaskList/ListFiles calls
  
  - [ ]* 6.7 Write property test for non-recursive listing
    - **Property 7: Non-recursive listing**
    - **Validates: Requirements 9.1, 9.2**
    - Generate nested directory structures
    - Verify no entry filename contains path separators

- [ ] 7. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Proto code generation requires buf CLI (run `buf generate` in proto/ directory)
- Best-effort enrichment means per-entry I/O errors are logged, not propagated
- Preview truncation must be rune-safe (100 characters, not bytes) to avoid splitting UTF-8
- Property-based tests use pgregory.net/rapid framework
- The project is pre-release, so no backward compatibility or migration code needed
