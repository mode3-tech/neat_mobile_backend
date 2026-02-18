# Neat Mobile App Backend

Go-based HTTP API backend using Gin, PostgreSQL, and JWT authentication.

## Purpose

This README documents the project structure and collaboration rules so contributors can add features without breaking architectural boundaries.

## Tech Stack

- Go `1.25.x`
- Gin (`github.com/gin-gonic/gin`)
- PostgreSQL (`database/sql` + `github.com/lib/pq`)
- JWT (`github.com/golang-jwt/jwt/v5`)
- Env loading (`github.com/joho/godotenv`)

## Project Structure

```text
neat_mobile_app_backend/
|-- cmd/
|   `-- api/
|       |-- main.go
|       `-- tmp/
|-- internal/
|   |-- config/
|   |   `-- config.go
|   |-- database/
|   |   `-- database.go
|   `-- server/
|       |-- router.go
|       `-- server.go
|-- modules/
|   `-- auth/
|       |-- dto.go
|       |-- handler.go
|       |-- interfaces.go
|       |-- respository.go
|       |-- routes.go
|       `-- service.go
|-- models/
|   `-- user.go
|-- pkg/
|   `-- jwt/
|       `-- signer.go
|-- database/
|-- middleware/
|-- errors/
|-- .env
|-- .env.example
|-- .gitignore
|-- go.mod
`-- go.sum
```

## Folder Responsibilities

### `cmd/`

Application entrypoints only.

- `cmd/api/main.go`:
  - Loads environment (`.env`) for local development.
  - Builds config.
  - Starts HTTP server.
  - Handles graceful shutdown on interrupt.

Rule: keep `cmd` thin. No business logic here.

### `internal/`

Private app wiring and infrastructure.

- `internal/config`: environment-to-config mapping.
- `internal/database`: PostgreSQL connection setup and health check (`Ping`).
- `internal/server`:
  - Constructs HTTP server with timeout settings.
  - Builds Gin router.
  - Wires module dependencies (`repo -> service -> handler -> routes`).

Rule: app composition belongs in `internal`, not in modules.

### `modules/`

Feature modules, organized by domain.

- `modules/auth` currently contains:
  - `dto.go`: request/response contracts.
  - `handler.go`: HTTP transport layer (Gin handlers).
  - `service.go`: business logic/use-cases.
  - `respository.go`: data access implementation.
  - `interfaces.go`: dependency abstractions used by service.
  - `routes.go`: route registration.

Rule: each module owns its transport, business logic, and persistence for that feature.

### `models/`

Shared domain entities used across modules.

- `models/user.go`: user entity fields.

Rule: keep models framework-agnostic where possible.

### `pkg/`

Reusable packages with cross-module utility value.

- `pkg/jwt/signer.go`: token generation and validation helpers.

Rule: `pkg` should remain generic and reusable.

### `database/`, `middleware/`, `errors/`

Currently placeholders for growth.

- `database/`: migrations, seed scripts, or SQL files.
- `middleware/`: shared HTTP middleware.
- `errors/`: centralized error types/helpers.

If these folders stay unused long-term, remove them to reduce noise.

## Request Lifecycle (Current)

1. `cmd/api/main.go` boots app.
2. `internal/server.New` creates router via `NewRouter`.
3. `internal/server/router.go` creates DB and module dependencies.
4. Auth routes are mounted under `/api/v1/auth`.
5. Handler validates input and delegates to service.
6. Service calls repository and JWT signer.
7. Handler returns JSON response.

## Environment Variables

Required:

- `PORT`: API port (example: `8080`)
- `DB_URL`: PostgreSQL DSN
- `JWT_SECRET`: signing key

Recommended:

- Keep real values out of version control.
- Use `.env.example` as the shared contract.
- Rotate any leaked secrets immediately.

## Local Run

```bash
go mod download
go run ./cmd/api
```

Default API base path:

- `http://localhost:<PORT>/api/v1`

## API Endpoints

Base URL:

- `http://localhost:<PORT>/api/v1`

### `POST /auth/login`

Authenticates a user and returns an access token.

Request body:

```json
{
  "email": "user@example.com",
  "password": "your-password"
}
```

Success response (`200 OK`):

```json
{
  "access_token": "<jwt-token>"
}
```

Validation and error responses:

- `400 Bad Request`: invalid request body
- `401 Unauthorized`: invalid credentials

### Logout Status

`Logout` handler exists in `modules/auth/handler.go` but route registration is not yet exposed in `modules/auth/routes.go`.

## Collaboration Rules

### 1) Add new features as modules

Create `modules/<feature>/` with:

- `dto.go`
- `handler.go`
- `service.go`
- `repository.go`
- `interfaces.go` (only if needed)
- `routes.go`

Wire it in `internal/server/router.go` only.

### 2) Keep clean dependency direction

Use this flow:

- `handler -> service -> repository`

Avoid reverse dependencies (e.g., repository importing handlers).

### 3) Keep transport concerns in handlers

- Parse/validate HTTP input in handlers.
- Business decisions in services.
- SQL/data concerns in repositories.

### 4) Shared utilities go to `pkg/`

Only place genuinely reusable logic in `pkg`.

### 5) Keep `internal/` as composition layer

Startup, config, DB wiring, and route composition stay in `internal`.

## Ownership Guidance

When editing, prefer these ownership boundaries:

- Routing/wiring: `internal/server/*`
- Feature behavior: `modules/<feature>/*`
- Shared types: `models/*`
- Cross-feature utility: `pkg/*`
- Runtime config: `internal/config/*`

Following these boundaries keeps PRs smaller, reviews faster, and regressions easier to isolate.
