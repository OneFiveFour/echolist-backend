---
applyTo: "proto/**/*.proto"
---

# Protobuf Review Rules

- All services use proto3 syntax.
- Package naming follows `{service}.v1` (e.g., `notes.v1`, `auth.v1`).
- `go_package` option must follow the pattern `echolist-backend/proto/gen/{service}/v1;{service}v1`.
- Enum values must start with `{ENUM_NAME}_UNSPECIFIED = 0`.
- Field numbering must never reuse or skip numbers in existing messages (buf breaking FILE mode is enforced).
- Request messages are named `{Rpc}Request`, response messages `{Rpc}Response`.
- Timestamps are represented as `int64` Unix milliseconds (`updated_at`), not `google.protobuf.Timestamp`.
- IDs are `string` fields containing lowercase hyphenated UUIDv4.
- Directory/path fields use forward slashes, relative to the data root. Empty string means root.
- Do not add streaming RPCs. All endpoints are unary.
- After any proto change, `proto/gen/` must be regenerated with `cd proto && buf generate`.
