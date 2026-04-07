# Docker Compose Deployment

## Prerequisites

- Docker and Docker Compose installed
- The `onefivefour/echolist-backend:latest` image pulled (or build locally with `docker build -t onefivefour/echolist-backend:latest .`)

## Configuration

Create a `.env` file in the project root:

```env
JWT_SECRET=your-secret-key-here
AUTH_DEFAULT_USER=admin
AUTH_DEFAULT_PASSWORD=a-strong-password
```

These two are required — the container will refuse to start without them:
- `JWT_SECRET` — signing key for access/refresh tokens
- `AUTH_DEFAULT_PASSWORD` — password for the initial user account

Optional env vars (with defaults):

| Variable | Default | Description |
|---|---|---|
| `AUTH_DEFAULT_USER` | `admin` | Initial username |
| `ACCESS_TOKEN_EXPIRY_MINUTES` | `15` | Access token TTL |
| `REFRESH_TOKEN_EXPIRY_MINUTES` | `10080` (7 days) | Refresh token TTL |
| `MAX_REQUEST_BODY_BYTES` | `4194304` (4 MB) | Max request body size |
| `SHUTDOWN_TIMEOUT_SECONDS` | `30` | Graceful shutdown timeout |

## Start

```bash
docker compose up -d
```

The API will be available at `http://localhost:9090`. The container listens on `8080` internally, mapped to `9090` on the host — change the port mapping in `docker-compose.yml` if needed.

## Persistent Data

Two volumes are mounted by default:

- `./data` → `/app/data` — notes, tasks, and file storage
- `./auth` → `/app/auth` — `users.json` credential store

Back these directories up as needed.

## Health Check

The compose file includes a health check against `/healthz`. Check status with:

```bash
docker compose ps
```

Or hit it directly:

```bash
curl http://localhost:9090/healthz
```

## Building Locally

If you're not pulling a pre-built image:

```bash
docker build -t onefivefour/echolist-backend:latest .
docker compose up -d
```

## Stopping

```bash
docker compose down
```

Add `-v` to also remove named volumes if applicable.
