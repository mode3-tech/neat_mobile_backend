# Neat Mobile App Backend

Go HTTP API backend for authentication, OTP, and device-security workflows using Gin, PostgreSQL, GORM, and JWT.

## Current Status

Implemented:

- User registration with transactional user creation and initial device bind
- Login with `phone + password + X-Device-ID`
- Device-aware login initialization responses:
  - `challenge_required`
  - `new_device_detected`
- Device challenge verification endpoint (`/auth/verify-device`) for recognized trusted devices
- Logout endpoint (session/refresh invalidation)
- Refresh token rotation (`/auth/refresh`)
- OTP request and OTP verify endpoints (`/auth/otp/request`, `/auth/otp/verify`)
- BVN and NIN validation endpoints
- In-memory login rate limiting with shard-based locking and bounded key capacity
- Auto migration on startup for auth + verification + device tables

Important current gap:

- Login is still missing OTP administration for new devices.
  - On `new_device_detected`, login returns a `session_token` but does not send OTP automatically yet.

## Runtime Architecture

Execution flow:

1. `cmd/api/main.go` loads environment and starts the HTTP server.
2. `internal/server/router.go` composes DB, providers, middleware, and routes.
3. `internal/database/database.go` opens Postgres and runs migrations.
4. Features follow `handler -> service -> repository`.
5. Transactional work uses `internal/database/tx/transactor.go`.

Main modules:

- `modules/auth`: registration, login init, logout, refresh, BVN/NIN
- `modules/auth/otp`: OTP request/verification
- `modules/auth/verification`: verification record persistence
- `modules/device`: device/challenge/pending-session persistence + challenge generation

## Project Structure (Current)

```text
neat_mobile_app_backend/
|-- cmd/api/
|   |-- main.go
|   `-- tmp/                   # local build artifacts
|-- docs/
|   |-- swagger.json
|   `-- swagger.yaml
|-- internal/
|   |-- adapters/cba/
|   |-- config/
|   |-- database/
|   |   |-- database.go
|   |   `-- tx/transactor.go
|   |-- middleware/
|   |   `-- login_rate_limit.go
|   |-- notify/
|   |-- server/
|   `-- validators/
|-- models/
|   |-- auth_session.go
|   |-- device_challenge.go
|   |-- pending_device_session.go
|   |-- refresh_token.go
|   |-- user.go
|   `-- verification_record.go
|-- modules/
|   |-- auth/
|   |   |-- dto.go
|   |   |-- handler.go
|   |   |-- helper.go
|   |   |-- interfaces.go
|   |   |-- models.go
|   |   |-- repository.go
|   |   |-- routes.go
|   |   |-- service.go
|   |   |-- types.go
|   |   |-- otp/
|   |   `-- verification/
|   `-- device/
|       |-- dto.go
|       |-- handler.go
|       |-- models.go
|       |-- repository.go
|       `-- service.go
|-- providers/
|   |-- bvn/
|   |-- email/
|   |-- jwt/
|   |-- nin/
|   `-- sms/
|-- templates/
|   `-- otp_email.html
|-- .env
|-- .env.example
|-- go.mod
`-- go.sum
```

## Auth API (Current Behavior)

Base URL: `http://localhost:<PORT>/api/v1`

### `POST /auth/register`

- Creates user + binds initial device in one transaction.
- On success returns access + refresh tokens.

### `POST /auth/login`

- Request body: `phone`, `password`
- Required header: `X-Device-ID`
- Returns:
  - `{"status":"challenge_required","challenge":"..."}`
  - or `{"status":"new_device_detected","session_token":"..."}`

Current limitation:

- `new_device_detected` path does not automatically send/administer OTP yet.

### `POST /auth/verify-device`

- Request body: `challenge`, `signature`, `device_id`
- Validates challenge existence/TTL/single-use, verifies signature against stored device public key, marks challenge used, updates `last_used_at`, then issues access + refresh tokens.

### `POST /auth/logout`

- Requires `Authorization: Bearer <access_token>` and `refresh_token` in body.

### `POST /auth/refresh`

- Validates refresh token, rotates token row, returns new access + refresh tokens.

### `POST /auth/validate-bvn`
### `POST /auth/validate-nin`

- Provider-backed identity validation endpoints.

### `POST /auth/otp/request`
### `POST /auth/otp/verify`

- OTP issuance and verification for `login`, `signup`, `password_reset`.

## Environment Variables

Core:

- `PORT`
- `DB_URL`
- `JWT_SECRET`

OTP and messaging:

- `PEPPER`
- `TERMII_APIKEY`
- `TERMII_SENDERID`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USER`
- `SMTP_PASS`

Identity providers:

- `TENDAR_APIKEY`
- `PREMBLY_APIKEY`
- `CBA_INTERNAL_URL`
- `CBA_INTERNAL_KEY`

Login rate limiter:

- `LOGIN_RATE_LIMIT_IP_MAX_ATTEMPTS`
- `LOGIN_RATE_LIMIT_EMAIL_MAX_ATTEMPTS` (name retained for backward compatibility; currently used for phone key budget in login limiter path)
- `LOGIN_RATE_LIMIT_WINDOW_MINUTES`
- `LOGIN_RATE_LIMIT_BLOCK_MINUTES`

## Database and Migrations

Startup migration (`internal/database/database.go`) currently includes:

- `models.User`
- `models.AuthSession`
- `models.RefreshToken`
- `models.VerificationRecord`
- `models.PendingDeviceSession`
- `modules/auth/otp.OTPModel`
- `modules/device.UserDevice`
- `modules/device.DeviceChallenge`

Also creates partial unique index:

- `uq_device_challenges_active` on `(user_id, device_id)` where `used_at IS NULL`

## Local Run

```bash
go mod download
go run ./cmd/api
```

## Dev Signer (Postman Testing)

Use this helper service to generate `signature` values for `/auth/verify-device` without a mobile client.

Set signer keys in `.env` (auto-loaded by `cmd/devsigner`):

```env
DEV_SIGNER_KEYS_JSON={"sim-device-1":"<p256_private_key_b64_or_hex>"}
# optional:
# DEV_SIGNER_ADDR=:9090
```

Run dev signer:

```bash
go run ./cmd/devsigner
```

Alternative config:

- `DEV_SIGNER_KEYS_FILE`: path to a JSON file of `{ "<device_id>": "<private_key>" }`
- `DEV_SIGNER_DEVICE_ID` + `DEV_SIGNER_PRIVATE_KEY`: single key pair
- `DEV_SIGNER_ADDR`: listen address (default `:9090`)
- Private key format: ECDSA P-256 private key (PEM/DER EC or PKCS8), or raw 32-byte scalar encoded as base64/base64url/hex

Request:

- `POST http://localhost:9090/sign`
- Body: `{"device_id":"sim-device-1","challenge":"<challenge-from-login>"}`
- Response: `{"signature":"...","algorithm":"ecdsa-p256-sha256"}`

Swagger:

- UI: `http://localhost:<PORT>/swagger/index.html`
- JSON: `http://localhost:<PORT>/openapi/doc.json`
- YAML: `http://localhost:<PORT>/openapi/doc.yaml`

## Known Gaps

- Login is missing OTP administration for new-device flow:
  - `new_device_detected` currently stops at issuing `session_token`.
  - Automatic OTP dispatch and follow-up new-device verification endpoint flow are not yet wired end-to-end.
