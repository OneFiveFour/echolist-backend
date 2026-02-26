# Echolist Backend

Echolist Backend is a file-based personal productivity server built with [Go](https://go.dev) and [ConnectRPC](https://connectrpc.com). It provides gRPC and HTTP APIs for managing notes, task lists, and folders — all persisted as plain files on disk. No database required.

## Services

The backend exposes four ConnectRPC services on a single HTTP/2 endpoint (default `:8080`):

### AuthService

JWT-based authentication with access and refresh tokens.

| RPC            | Description                                      |
|----------------|--------------------------------------------------|
| `Login`        | Authenticate with username/password, receive tokens |
| `RefreshToken` | Exchange a refresh token for a new access token  |

All other service endpoints are protected by the auth interceptor.

### NoteService

Markdown notes organized in a folder hierarchy.

| RPC          | Description                          |
|--------------|--------------------------------------|
| `CreateNote` | Create a new note at a given path    |
| `GetNote`    | Retrieve a single note by file path  |
| `ListNotes`  | List notes and subfolders in a path  |
| `UpdateNote` | Update the content of an existing note |
| `DeleteNote` | Delete a note                        |

### FolderService

Manage the folder structure used by notes and task lists.

| RPC            | Description                            |
|----------------|----------------------------------------|
| `CreateFolder` | Create a new folder under a parent     |
| `GetFolder`    | Get folder metadata                    |
| `ListFolders`  | List child folders of a parent         |
| `UpdateFolder` | Rename a folder                        |
| `DeleteFolder` | Delete a folder                        |

### TaskListService

Task lists with support for subtasks, due dates, and recurrence rules (RRULE).

| RPC              | Description                          |
|------------------|--------------------------------------|
| `CreateTaskList` | Create a new task list at a path     |
| `GetTaskList`    | Retrieve a task list by file path    |
| `ListTaskLists`  | List task lists and entries in a path |
| `UpdateTaskList` | Update tasks within a list           |
| `DeleteTaskList` | Delete a task list                   |

## Getting Started

### Prerequisites

- Go 1.24+
- Docker and Docker Compose (optional, for containerized deployment)

### Run locally

```bash
export JWT_SECRET="your-secret-key"
export AUTH_DEFAULT_PASSWORD="your-admin-password"
go run .
```

### Run with Docker Compose

```bash
export JWT_SECRET="your-secret-key"
export AUTH_DEFAULT_PASSWORD="your-admin-password"
docker compose up
```

### Environment Variables

| Variable                       | Required | Default   | Description                          |
|--------------------------------|----------|-----------|--------------------------------------|
| `JWT_SECRET`                   | yes      | —         | Secret key for signing JWTs          |
| `AUTH_DEFAULT_PASSWORD`        | yes      | —         | Password for the default user        |
| `DATA_DIR`                     | no       | `./data`  | Directory for file-based storage     |
| `AUTH_DEFAULT_USER`            | no       | `admin`   | Username for the default user        |
| `ACCESS_TOKEN_EXPIRY_MINUTES`  | no       | `15`      | Access token lifetime in minutes     |
| `REFRESH_TOKEN_EXPIRY_MINUTES` | no       | `10080`   | Refresh token lifetime in minutes (default 7 days) |

### gRPC Reflection

The server enables gRPC reflection, so you can explore the API with tools like [grpcurl](https://github.com/fullstorydev/grpcurl) or [Buf Studio](https://buf.build/studio).

## Running Tests

```bash
go test ./...
```

## Contributing

Contributions are welcome. To get started:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Commit your changes (`git commit -m 'Add my feature'`)
4. Push to the branch (`git push origin feature/my-feature`)
5. Open a Pull Request

Please make sure your code passes all existing tests (`go test ./...`) and follows the existing code style. If you're adding a new feature, include tests for it.

For bugs and feature requests, please [open an issue](../../issues).

## License

This project is licensed under the GNU General Public License v3.0. See the [LICENSE](LICENSE) file for details.
