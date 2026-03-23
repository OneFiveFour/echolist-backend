# Copilot Review Instructions — echolist-backend

## Project Overview

echolist-backend is a personal productivity backend (notes, tasks, file/folder management) written in Go 1.24. It uses Connect RPC (gRPC-compatible) over HTTP/2, stores all data as plain files on disk (no database), and authenticates via JWT. The codebase is intentionally small and dependency-light.

## Technology Stack

- Go 1.24 (module `echolist-backend`)
- Connect RPC (`connectrpc.com/connect` v1.19) for all service endpoints
- Protocol Buffers v3 with `buf` (v2 config) for code generation
- JWT authentication (`github.com/golang-jwt/jwt/v5`, HS256)
- bcrypt password hashing (`golang.org/x/crypto`)
- Property-based testing with `pgregory.net/rapid`
- Structured logging via `log/slog` (JSON handler)
- Multi-stage Docker build (golang:1.24-alpine → alpine)

## Architecture & Package Layout

```
main.go                  — HTTP server setup, middleware, health probes, graceful shutdown
auth/                    — JWT token service, user store (file-based), auth interceptor
  auth_server.go         — Login / RefreshToken RPC handlers
  interceptor.go         — Unary interceptor: validates Bearer tokens, injects username into ctx
  token_service.go       — JWT generation & validation (access + refresh tokens)
  user_store.go          — File-based credential store (users.json, bcrypt)
file/                    — Folder CRUD and ListFiles (enriched directory listing)
  file_server.go         — FileServer struct (embeds UnimplementedFileServiceHandler)
  create_folder.go       — CreateFolder RPC
  list_files.go          — ListFiles RPC (reads children, builds metadata per item type)
  update_folder.go       — UpdateFolder (rename) RPC
  delete_folder.go       — DeleteFolder RPC
notes/                   — Note CRUD with UUID registry
  notes_server.go        — NotesServer struct
  create_note.go         — CreateNote (atomic exclusive create, UUID generation, registry)
  update_note.go         — UpdateNote (registry lookup → atomic overwrite)
  get_note.go            — GetNote (registry lookup → read file)
  delete_note.go         — DeleteNote (registry remove + file delete)
  list_notes.go          — ListNotes (directory scan)
  registry.go            — JSON-based id→filePath registry (.note_id_registry.json)
  uuid.go                — UUIDv4 generation & validation
  title.go               — Title extraction from note filenames
tasks/                   — Task list CRUD with markdown-based storage
  task_server.go         — TaskServer struct, proto ↔ domain converters
  parser.go              — Markdown task file parser (checkbox format)
  printer.go             — Markdown task file serializer
  create_task_list.go    — CreateTaskList RPC
  get_task_list.go       — GetTaskList RPC
  update_task_list.go    — UpdateTaskList RPC
  delete_task_list.go    — DeleteTaskList RPC
  list_task_lists.go     — ListTaskLists RPC
common/                  — Shared utilities
  pathutil.go            — Path validation, traversal prevention, name validation, file type matching
  atomicwrite.go         — Atomic file writes (temp + rename), exclusive create
  pathlock.go            — Per-path mutex (Locker)
  logging.go             — Request logging interceptor
proto/                   — Protobuf definitions and generated code
  auth/v1/auth.proto     — AuthService (Login, RefreshToken)
  file/v1/file.proto     — FileService (CreateFolder, ListFiles, UpdateFolder, DeleteFolder)
  notes/v1/notes.proto   — NoteService (CreateNote, ListNotes, GetNote, UpdateNote, DeleteNote)
  tasks/v1/tasks.proto   — TaskListService (CRUD for task lists)
  gen/                   — Generated Go code (DO NOT edit manually)
  buf.yaml               — Buf lint (STANDARD) and breaking (FILE) config
  buf.gen.yaml           — Buf code generation config (protoc-gen-go + protoc-gen-connect-go)
```

## Build & Test

```bash
# Build
go build -o echolist-backend .

# Run all tests (includes property-based tests via rapid)
go test ./...

# Run tests for a single package
go test ./notes/...

# Docker build
docker build -t echolist-backend .

# Protobuf code generation (requires buf, protoc-gen-go, protoc-gen-connect-go)
cd proto && buf generate
```

There is no Makefile, no CI pipeline, and no linter config beyond `buf lint`. Tests use `go test` only.

## Code Conventions to Enforce

### Error Handling
- All RPC errors MUST use `connect.NewError(connect.Code*, ...)` with appropriate gRPC status codes.
- Internal errors should be logged with `s.logger.Error(...)` before returning a generic `CodeInternal` error to the client. Never leak internal details (file paths, stack traces) to callers.
- Wrap errors with `fmt.Errorf("context: %w", err)` to preserve the error chain.
- Use `errors.Is()` for sentinel checks (e.g., `os.ErrNotExist`, `os.ErrExist`).

### Path Safety & Validation
- Every user-supplied path MUST go through `common.ValidatePath()` or `common.ValidateParentDir()` before any filesystem operation. These functions resolve symlinks and reject path traversal.
- User-supplied names (titles, folder names) MUST be validated with `common.ValidateName()` which rejects path separators, `.`/`..`, null bytes, and names exceeding 255 bytes.
- Content size MUST be checked with `common.ValidateContentLength()` before writing.

### Concurrency & File I/O
- All file mutations MUST acquire a per-path lock via `s.locks.Lock(path)` and defer the returned unlock function.
- File writes MUST use `common.File()` (atomic temp+rename) or `common.CreateExclusive()` (atomic exclusive create). Never use `os.WriteFile()` directly for data files.
- The note registry (`registryAdd`, `registryRemove`) requires its own lock on `registryPath(s.dataDir)`.

### Server & Handler Patterns
- Each service package has a server struct embedding the `Unimplemented*Handler` from the generated connect code.
- Server constructors follow the pattern: `NewXxxServer(dataDir string, logger *slog.Logger) *XxxServer` with `logger.With("service", "xxx")`.
- RPC handler methods are in separate files named after the operation (e.g., `create_note.go`, `list_files.go`).

### Protobuf
- Proto files live under `proto/{service}/v1/{service}.proto`.
- Generated code lives under `proto/gen/` and must never be edited by hand.
- Use `req.GetField()` (getter methods) to access proto fields, never direct field access on request messages.
- Buf lint uses STANDARD rules; buf breaking uses FILE mode.

### Testing
- Property-based tests use `pgregory.net/rapid` and follow the naming convention `TestProperty[N]_DescriptiveName` or `TestProperty_DescriptiveName`.
- Custom generators are named `xxxGen()` returning `*rapid.Generator[T]` (e.g., `usernameGen()`, `secretGen()`, `titleGen()`).
- Each test package provides a `nopLogger()` helper that returns a discard logger.
- The `auth` package uses `TestMain` to lower `bcryptCost` to `bcrypt.MinCost` for fast test execution.
- Tests use `t.TempDir()` for isolated filesystem state. Never write to the real `data/` or `auth/` directories.
- Test files are colocated with source in the same package (white-box testing).

### Logging
- Use `log/slog` structured logging exclusively. No `fmt.Println` or `log.Printf`.
- Logger is passed via constructor injection, never as a global.
- Use `slog.With("key", "value")` for contextual fields. Service-level loggers add `"service"` key.

### Authentication & Security
- The auth interceptor (`auth.NewAuthInterceptor`) protects all RPCs except those in `publicProcedures` (Login, RefreshToken).
- Tokens carry a `type` claim (`"access"` or `"refresh"`). Only access tokens are accepted for API calls; refresh tokens are only valid at the RefreshToken endpoint.
- Passwords are hashed with bcrypt (cost 10 in production). The `bcryptCost` variable is intentionally mutable for test performance.
- User credentials file uses 0600 permissions. The user store directory is created with 0700.

### File Naming Conventions
- Notes: `note_{title}.md`
- Task lists: `tasks_{title}.md`
- These conventions are encoded in `common.NoteFileType` and `common.TaskListFileType` and matched via `common.MatchesFileType()`.

### What NOT to Do
- Do not add a database or ORM. The text-file storage model is intentional.
- Do not introduce global mutable state. Configuration flows through constructors and environment variables.
- Do not add middleware that isn't a Connect interceptor (except the `maxBytesMiddleware` in main.go).
- Do not modify files under `proto/gen/`. Regenerate with `buf generate` instead.
- Do not use `context.Background()` in RPC handlers. Always propagate the incoming `ctx`.
