# Persistent device sessions

## Implemented design

1. Each password login creates a separate device session in SQLite (`auth_sessions`
   in the existing `DATA_DIR/echolist.db`). The client supplies a display name;
   this is a label, not a verified device identity. Repeated logins create separate
   sessions, even on the same device.
2. Access tokens last 15 minutes by default. Refresh tokens and their session last
   90 days. A successful refresh rotates the refresh token and moves expiry to
   90 days from that refresh. This is an inactivity limit, with no absolute login
   lifetime. Android background refreshes also count as activity.
3. Every protected request checks the persistent session, so revocation blocks both
   access and refresh tokens on the next request. Requests already authorized may
   finish. Expired/revoked sessions cannot be renewed. A database failure fails
   closed with a server error and does not silently log out clients.
4. A 60-second overlap permits recovery after a lost refresh response and concurrent
   refreshes from browser tabs/workers. During this interval both the current and
   immediately previous token return the same current refresh token without further
   extension. Using an older, still cryptographically valid refresh token outside
   that window revokes the session. No bearer tokens are persisted in the database;
   only session metadata and random token identifiers are stored.
5. Android/JS/Wasm/Desktop share rotated-token persistence, refresh request
   deduplication and error handling. Only a 401 from refresh clears credentials.
   Network errors, timeouts and server errors keep the session for a later retry.

## Configuration and rollout

- `ACCESS_TOKEN_EXPIRY_MINUTES=15`
- `REFRESH_TOKEN_EXPIRY_MINUTES=129600` (90 days; existing explicit overrides win)
- Keep `JWT_SECRET` stable across restarts and backend instances.
- Deploy the updated clients **before** or together with the backend. The new client
  also accepts old backend refresh responses without a new refresh token. Old clients
  do not store rotated refresh tokens and are incompatible with the new backend.
- After upgrading the backend, sign in once per device. Sessionless legacy JWTs are
  deliberately rejected; they cannot provide immediate per-device revocation.
- Back up `echolist.db` together with the data. The schema migration is additive.
  Losing this database invalidates all sessions. Session metadata/tombstones remain
  stored; active-session listing excludes expired and revoked rows.
- Serve public deployments over HTTPS. Browser tokens still use the existing
  localStorage implementation; HttpOnly cookie migration is a separate change.
- As before, changing `AUTH_DEFAULT_PASSWORD` does not replace an existing user's
  password. Use session revocation to block existing device access.

## Manage sessions from the terminal

No additional Python packages are required. The tool asks for the account password,
keeps credentials only in memory, and revokes its temporary management session on exit.
It manages sessions belonging to that account; it cannot revoke another user's devices.

```sh
python3 scripts/sessions.py --url https://notes.example.org --username admin list
python3 scripts/sessions.py --url https://notes.example.org --username admin revoke DEVICE_SESSION_UUID
```

The list includes the ID, device label, last refresh and expiry in UTC. Management
is exposed through API and terminal; there is no new device-management screen in the app.

## API

Connect JSON POST requests use `Content-Type: application/json` and
`Connect-Protocol-Version: 1`. List/revoke require `Authorization: Bearer ACCESS_TOKEN`.
Timestamps are Unix milliseconds (encoded as strings in protobuf JSON).

- `/auth.v1.AuthService/Login`: optional `deviceName` (maximum 200 UTF-8 bytes).
- `/auth.v1.AuthService/RefreshToken`: returns `accessToken` **and** `refreshToken`.
  Persist the refresh token before publishing the access token.
- `/auth.v1.AuthService/ListSessions`: `{}` → `sessions` containing `id`,
  `deviceName`, `createdAt`, `lastUsedAt`, `expiresAt`, `isCurrent`.
- `/auth.v1.AuthService/RevokeSession`: `{"sessionId":"UUID"}` → `{}`.
  Idempotent; unknown or other-account IDs do not reveal session existence.

## Verification

Regression coverage includes session persistence, expiry, sliding renewal, rotation,
recovery during overlap, replay revocation, account isolation and HTTP authorization,
including immediate access-token rejection after revocation and legacy-token rejection.
Client tests cover rotated-token storage, concurrent unauthorized requests and preserving
credentials during network/timeout/server failures. Compile the shared client module for
Android, JS and WasmJS as well as running its JVM tests and architecture check.
