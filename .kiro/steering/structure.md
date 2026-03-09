---
inclusion: auto
---

# Project Structure

## Top-Level Organization

```
echolist-backend/
├── auth/           # Authentication service implementation
├── common/         # Shared utilities (logging, path handling, atomic writes)
├── file/           # File/folder service implementation
├── notes/          # Notes service implementation
├── tasks/          # Task list service implementation
├── proto/          # Protocol Buffer definitions and generated code
├── data/           # Runtime data directory (not committed)
├── .kiro/          # Kiro configuration and specs
├── main.go         # Server entry point and wiring
└── go.mod          # Go module definition
```

## Service Package Layout

Each service package (`auth/`, `file/`, `notes/`, `tasks/`) follows this pattern:

- `*_server.go`: Connect RPC service implementation
- `*_server_test.go`: Unit tests for the service
- `*_property_test.go`: Property-based tests using rapid
- `test_helpers_test.go`: Shared test utilities
- `testdata/rapid/`: Property test failure examples (auto-generated)
- Individual operation files (e.g., `create_note.go`, `list_files.go`)

## Proto Organization

```
proto/
├── auth/v1/auth.proto       # Auth service definitions
├── file/v1/file.proto       # File service definitions
├── notes/v1/notes.proto     # Notes service definitions
├── tasks/v1/tasks.proto     # Tasks service definitions
├── buf.yaml                 # Buf configuration
├── buf.gen.yaml             # Code generation config
└── gen/                     # Generated Go code
    ├── auth/v1/
    ├── file/v1/
    ├── notes/v1/
    └── tasks/v1/
```

## Common Package

Shared utilities used across services:

- `atomicwrite.go`: Atomic file write operations
- `pathlock.go`: Path-based locking for concurrent access
- `pathutil.go`: Path validation and canonicalization
- `logging.go`: Request logging interceptor
- Validation helpers for names, file types, directories

## Testing Conventions

### Test File Naming
- `*_test.go`: Standard unit tests
- `*_property_test.go`: Property-based tests using rapid
- `test_helpers_test.go`: Test utilities (not exported)

### Property-Based Testing
- Uses `pgregory.net/rapid` for generative testing
- Test failures saved to `testdata/rapid/TestName/` directories
- Properties validate correctness requirements from specs
- Each property test includes a comment linking to the spec requirement

### Test Helpers
- `nopLogger()`: No-op logger for tests
- Custom generators for valid inputs (e.g., `nameGen()`, `usernameGen()`)
- Shared assertion helpers

## File Naming Conventions

### Notes
- Stored as `note_<title>.md` in the data directory
- Title extracted from filename by removing `note_` prefix and `.md` suffix

### Task Lists
- Stored as `tasks_<title>.md` in the data directory
- Title extracted from filename by removing `tasks_` prefix and `.md` suffix

### Folders
- Created directly in the data directory hierarchy
- No special prefix required

## Configuration Files

- `.dockerignore`: Docker build exclusions
- `.gitignore`: Git exclusions
- `docker-compose.yml`: Docker Compose service definition
- `Dockerfile`: Multi-stage Docker build
- `buf.lock`: Buf dependency lock file
- `go.mod`, `go.sum`: Go module dependencies

## Specs Directory

`.kiro/specs/` contains feature and bugfix specifications following the spec-driven development methodology. Each spec includes requirements, design, and task breakdown.
