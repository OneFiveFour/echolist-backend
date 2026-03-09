---
inclusion: auto
---

# Product Overview

Echolist Backend is a gRPC/Connect-based API server for managing personal knowledge and task data. It provides four core services:

- **Notes Service**: Create, read, update, delete, and list markdown notes organized in folders
- **Tasks Service**: Manage task lists with support for subtasks, due dates, and recurrence rules
- **File Service**: CRUD operations for folder management within the data directory
- **Auth Service**: JWT-based authentication with access and refresh tokens

The system stores all user data as files in a configurable data directory, using markdown format for notes and task lists. Authentication credentials are stored separately in bcrypt-hashed JSON format.

## Key Design Principles

- File-based storage with no external database dependencies
- JWT authentication with separate access and refresh tokens
- Structured logging with slog
- Property-based testing for correctness validation
- Connect RPC protocol (gRPC-compatible HTTP/2)
