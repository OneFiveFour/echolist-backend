# Tasks: API Hardening & Cleanup

## Task 1: Fix ListFiles task file filter prefix
- [ ] 1.1 In `file/list_files.go`, change `"task_"` prefix check to `"tasks_"` in the file filter loop
- [ ] 1.2 Write property test `TestProperty_ListFilesFilterCorrectness` in `file/list_files_property_test.go` using `rapid` — generate random directory contents with `note_`, `tasks_`, other-prefixed files and subdirectories, verify ListFiles returns exactly the expected entries
  - PBT: Feature: api-hardening-cleanup, Property 1: ListFiles filter correctness

## Task 2: Remove entries field from ListNotesResponse
- [ ] 2.1 Remove `repeated string entries = 2` from `ListNotesResponse` in `proto/notes/v1/notes.proto`
- [ ] 2.2 Run `buf generate` to regenerate Go code
- [ ] 2.3 Update `server/listNotes.go` — remove `entries` slice building, remove `Entries` from response, skip directory entries in the loop
- [ ] 2.4 Update `server/listNotes_test.go` and `server/listNotes_property_test.go` to remove references to `Entries` field
- [ ] 2.5 Write property test `TestProperty_ListNotesExcludesDirectories` in `server/listNotes_property_test.go` — generate directories with notes and subdirectories, verify ListNotes returns only Note objects
  - PBT: Feature: api-hardening-cleanup, Property 2: ListNotes returns only notes, no directory entries

## Task 3: Add path traversal protection to NoteService handlers
- [ ] 3.1 In `server/createNote.go`, add `pathutil.IsSubPath` validation on `req.Path` (directory), matching `CreateTaskList` pattern
- [ ] 3.2 In `server/getNote.go`, add `pathutil.ValidatePath` validation on `req.FilePath`, matching `GetTaskList` pattern
- [ ] 3.3 In `server/updateNote.go`, add `pathutil.ValidatePath` validation on `req.FilePath`, matching `UpdateTaskList` pattern
- [ ] 3.4 In `server/deleteNote.go`, add `pathutil.ValidatePath` validation on `req.FilePath`, matching `DeleteTaskList` pattern
- [ ] 3.5 In `server/listNotes.go`, add `pathutil.IsSubPath` validation on `req.Path` (directory), matching `ListTaskLists` pattern
- [ ] 3.6 Write property test `TestProperty_PathTraversalRejection` in `server/pathTraversal_property_test.go` — generate path traversal strings, call each handler, verify all return `CodeInvalidArgument`
  - PBT: Feature: api-hardening-cleanup, Property 3: Path traversal rejection across NoteService

## Task 4: Use proper Connect error codes in NoteService
- [ ] 4.1 In `server/getNote.go`, replace raw error returns with `connect.NewError` — use `CodeNotFound` for `os.ErrNotExist`, `CodeInternal` for other I/O errors
- [ ] 4.2 In `server/deleteNote.go`, replace raw error return with `connect.NewError` — use `CodeNotFound` for `os.ErrNotExist`, `CodeInternal` for other errors
- [ ] 4.3 In `server/createNote.go`, replace `fmt.Errorf` returns with `connect.NewError(connect.CodeInternal, ...)`
- [ ] 4.4 In `server/updateNote.go`, replace `fmt.Errorf` returns with `connect.NewError(connect.CodeInternal, ...)`
- [ ] 4.5 In `server/listNotes.go`, replace `fmt.Errorf` returns with `connect.NewError(connect.CodeInternal, ...)`
- [ ] 4.6 Write property test `TestProperty_NotFoundReturnsCodeNotFound` in `server/notFound_property_test.go` — generate random non-existent file paths, verify GetNote and DeleteNote return `CodeNotFound`
  - PBT: Feature: api-hardening-cleanup, Property 4: Non-existent file returns CodeNotFound
- [ ] 4.7 Update existing NoteService unit tests to assert Connect error codes instead of raw errors

## Task 5: Add input validation to CreateNote
- [ ] 5.1 In `server/createNote.go`, add empty title validation returning `CodeInvalidArgument` with message "title must not be empty"
- [ ] 5.2 In `server/createNote.go`, add path separator validation (`/` and `\`) returning `CodeInvalidArgument` with message "title must not contain path separators"
- [ ] 5.3 Write property test `TestProperty_TitleWithPathSeparatorsRejected` in `server/createNote_property_test.go` — generate titles containing `/` or `\`, verify CreateNote returns `CodeInvalidArgument`
  - PBT: Feature: api-hardening-cleanup, Property 5: Titles containing path separators are rejected
- [ ] 5.4 Add unit test for empty title rejection in `server/createNote_test.go`

## Task 6: Extract atomicWriteFile to shared package
- [ ] 6.1 Create `atomicwrite/atomicwrite.go` with exported `File(path string, data []byte) error` function
- [ ] 6.2 Move `server/atomicWriteFile_test.go` to `atomicwrite/atomicwrite_test.go`, update package and imports
- [ ] 6.3 Delete `server/atomicWriteFile.go` and update `server/createNote.go` and `server/updateNote.go` to import and call `atomicwrite.File`
- [ ] 6.4 Remove `atomicWriteFile` function from `tasks/task_server.go` and update `tasks/create_task_list.go` and `tasks/update_task_list.go` to import and call `atomicwrite.File`
- [ ] 6.5 Run all tests to verify NoteService and TaskService still pass
