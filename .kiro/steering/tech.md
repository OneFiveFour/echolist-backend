---
inclusion: auto
---

# Technology Stack

## Language & Runtime
- **Go 1.24** with toolchain 1.24.13
- Standard library for most operations

## Naming Conventions
- Acronyms are treated as regular words in identifiers and comments: use `Id`, `Url`, `Http`, `Api` — not `ID`, `URL`, `HTTP`, `API`
- Examples: `noteId`, `userId`, `buildTaskListIds`, `"failed to persist note id"`

## Core Dependencies
- **connectrpc.com/connect**: Connect RPC framework (gRPC-compatible)
- **connectrpc.com/grpcreflect**: gRPC reflection for debugging
- **google.golang.org/protobuf**: Protocol Buffers
- **github.com/golang-jwt/jwt/v5**: JWT token generation and validation
- **golang.org/x/crypto**: bcrypt password hashing
- **pgregory.net/rapid**: Property-based testing framework
- **github.com/teambition/rrule-go**: Recurrence rule parsing

## Protocol Buffers
- Proto definitions in `proto/*/v1/*.proto`
- Generated code in `proto/gen/*/v1/`
- Uses buf for proto management (`buf.yaml`, `buf.gen.yaml`)

## Testing Framework
- Standard Go testing (`testing` package)
- Property-based tests using `pgregory.net/rapid`
- Test data stored in `testdata/rapid/` directories

## Build & Development

### Common Commands

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...

# Run tests in a specific package
go test ./notes

# Build the binary
go build -o echolist-backend .

# Run the server locally
DATA_DIR=./data JWT_SECRET=your-secret-here AUTH_DEFAULT_PASSWORD=admin123 ./echolist-backend

# Generate proto code (requires buf)
cd proto && buf generate

# Build Docker image
docker build -t echolist-backend .

# Run with Docker Compose
docker-compose up
```

### Environment Variables

Required:
- `JWT_SECRET`: Secret key for JWT signing
- `AUTH_DEFAULT_PASSWORD`: Initial admin password

Optional:
- `DATA_DIR`: Data storage directory (default: `./data`)
- `AUTH_DEFAULT_USER`: Initial username (default: `admin`)
- `ACCESS_TOKEN_EXPIRY_MINUTES`: Access token TTL (default: 15)
- `REFRESH_TOKEN_EXPIRY_MINUTES`: Refresh token TTL (default: 10080 = 7 days)
- `MAX_REQUEST_BODY_BYTES`: Request size limit (default: 4194304 = 4MB)
- `SHUTDOWN_TIMEOUT_SECONDS`: Graceful shutdown timeout (default: 30)

## Server Configuration
- HTTP/2 with h2c (HTTP/2 cleartext) support
- Port 8080 (configurable via address in main.go)
- Structured JSON logging via slog
- Health endpoints: `/livez` (liveness), `/healthz` (readiness)
- gRPC reflection enabled for debugging
