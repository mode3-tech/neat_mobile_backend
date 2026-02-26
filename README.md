# Neat Mobile App Backend

Go HTTP API backend for authentication and OTP workflows, built with Gin, PostgreSQL, GORM, JWT, and SMS/SMTP providers.

## Current Stage

This project currently includes:

- Auth login with JWT access + refresh token issuance
- Logout endpoint that invalidates session/refresh token records
- Refresh token rotation logic in the service/repository layer
- OTP request/verify flows (SMS and email channels)
- PostgreSQL-backed session, refresh token, and OTP persistence
- Automatic GORM migrations on startup

Current implementation notes:

- `POST /auth/refresh` is registered, but the HTTP handler is still incomplete
- OTP email delivery is implemented over SMTP (TLS/STARTTLS) and depends on correct DNS + a trusted mail certificate

## Tech Stack

- Go `1.25.5`
- Gin (`github.com/gin-gonic/gin`)
- GORM (`gorm.io/gorm`)
- PostgreSQL (GORM Postgres driver `gorm.io/driver/postgres`, `github.com/lib/pq`)
- JWT (`github.com/golang-jwt/jwt/v5`, HS256 signing)
- UUIDs (`github.com/google/uuid`)
- Password hashing (`golang.org/x/crypto/bcrypt`)
- Env loading (`github.com/joho/godotenv`)
- SMTP email (`net/smtp`, `crypto/tls`, `html/template`)
- SMS provider integration (SMSLive247 HTTP API)
- Testing (`github.com/DATA-DOG/go-sqlmock`)

## Project Structure

```text
neat_mobile_app_backend/
|-- cmd/
|   `-- api/
|       |-- main.go
|       `-- tmp/                  # local build artifacts (generated)
|-- internal/
|   |-- config/
|   |   `-- config.go
|   |-- database/
|   |   `-- database.go
|   |-- notify/
|   |   `-- interfaces.go
|   `-- server/
|       |-- router.go
|       `-- server.go
|-- models/
|   |-- auth_session.go
|   |-- refresh_token.go
|   `-- user.go
|-- modules/
|   `-- auth/
|       |-- dto.go
|       |-- handler.go
|       |-- interfaces.go
|       |-- repository.go
|       |-- repository_test.go
|       |-- routes.go
|       |-- service.go
|       |-- types.go
|       `-- otp/
|           |-- dto.go
|           |-- handler.go
|           |-- helper.go
|           |-- otp_model.go
|           |-- repository.go
|           |-- routes.go
|           |-- service.go
|           `-- types.go
|-- providers/
|   |-- email/
|   |   `-- email.go
|   |-- jwt/
|   |   `-- signer.go
|   `-- sms/
|       `-- sms.go
|-- templates/
|   `-- otp_email.html
|-- .env
|-- .env.example
|-- README.md
|-- go.mod
`-- go.sum
```

## Architecture (Current)

The app follows a simple composition pattern:

1. `cmd/api/main.go` loads env vars and starts the HTTP server
2. `internal/server.New` builds the server and timeouts
3. `internal/server/router.go` wires config, DB, providers, and routes
4. `internal/database/database.go` opens PostgreSQL and runs `AutoMigrate`
5. Feature routes delegate to `handler -> service -> repository`

Dependency roles:

- `internal/*`: app composition, config, DB setup, server wiring
- `modules/*`: feature HTTP handlers, business logic, and persistence behavior
- `providers/*`: external integrations (JWT, SMS, email)
- `models/*`: shared DB model structs
- `templates/*`: HTML email templates

## Environment Variables

Core app config:

- `PORT`: API port (example: `8080`)
- `DB_URL`: PostgreSQL DSN
- `JWT_SECRET`: JWT signing secret

OTP and provider config:

- `PEPPER`: HMAC pepper used to hash OTP values
- `SMSLIVE_APIKEY`: SMSLive247 API key
- `SMSLIVE_SENDERID`: SMS sender ID
- `SMTP_HOST`: SMTP hostname (must resolve in DNS)
- `SMTP_PORT`: SMTP port (`465` for implicit TLS, `587` commonly used for STARTTLS)
- `SMTP_USER`: SMTP username / sender account
- `SMTP_PASS`: SMTP password

Recommendations:

- Keep real values out of version control
- Use `.env.example` as the shared contract
- Rotate any leaked secrets immediately

## SMTP TLS/DNS Notes (OTP Email)

The OTP email sender validates the SMTP server certificate. Misconfigured mail DNS or certificates commonly cause:

- `TLS connection failed: tls: failed to verify certificate: x509: certificate signed by unknown authority`
- `TLS connection failed: dial tcp: lookup <host>: no such host`

Use these rules for `SMTP_HOST`:

- Prefer a hostname (for example `mail.example.com`), not a raw IP address
- The hostname in `SMTP_HOST` must resolve in public DNS (`A` or `AAAA`)
- The SMTP certificate must match that hostname (CN/SAN)
- The certificate should be signed by a trusted CA (self-signed certs fail by default)
- Avoid using a VPS panel hostname unless you also publish DNS for it and install a trusted mail certificate

Examples:

- Good: `SMTP_HOST=mail.neatmicrocredit.com.ng`
- Risky: `SMTP_HOST=209.74.88.150` (TLS name mismatch if cert is issued to a hostname)
- Broken: `SMTP_HOST=server1.www.neatmicrocredit.xyz` when no public DNS record exists

## Local Run

```bash
go mod download
go run ./cmd/api
```

Default base path:

- `http://localhost:<PORT>/api/v1`

Startup behavior:

- Loads `.env` if present
- Builds router + providers
- Runs GORM migrations (`User`, `AuthSession`, `RefreshToken`, `OTPModel`)
- Starts HTTP server with graceful shutdown on `Ctrl+C`

## API Endpoints (Current)

Base URL:

- `http://localhost:<PORT>/api/v1`

### `POST /auth/login`

Authenticates a user and returns access and refresh tokens.

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
  "access_token": "<jwt-access-token>",
  "refresh_token": "<jwt-refresh-token>"
}
```

Common errors:

- `400 Bad Request`: invalid request body
- `401 Unauthorized`: invalid email/password

### `POST /auth/logout`

Revokes the current auth session + refresh token record.

Headers:

- `Authorization: Bearer <access-token>`

Request body:

```json
{
  "refresh_token": "<jwt-refresh-token>"
}
```

Success response (`200 OK`):

```json
{
  "message": "logout successful"
}
```

### `POST /auth/refresh`

Route is registered, but the current handler is incomplete and does not yet return rotated tokens over HTTP.

Current expected request body shape:

```json
{
  "refresh_token": "<jwt-refresh-token>"
}
```

### `POST /auth/otp/request`

Requests an OTP for login/signup/password reset over SMS or email.

Request body:

```json
{
  "purpose": "login",
  "channel": "email",
  "destination": "user@example.com"
}
```

Supported values:

- `purpose`: `login`, `signup`, `password_reset`
- `channel`: `sms`, `email`

Success response (`200 OK`):

```json
{
  "message": "otp sent"
}
```

### `POST /auth/otp/verify`

Verifies a previously issued OTP.

Request body:

```json
{
  "purpose": "login",
  "channel": "email",
  "destination": "user@example.com",
  "otp": "123456"
}
```

Success response (`200 OK`):

```json
{
  "message": "otp verified"
}
```

Common error responses include:

- `400 Bad Request`: invalid request body / invalid purpose / invalid channel / invalid email or phone
- `401 Unauthorized`: invalid OTP
- `429 Too Many Requests`: cooldown/resend limit reached
- `502 Bad Gateway`: SMS provider delivery failure
- `503 Service Unavailable`: SMS provider not configured

## OTP Behavior (Current)

Implemented OTP rules in `modules/auth/otp/*`:

- 6-digit OTP generation
- HMAC-SHA256 OTP hashing using `PEPPER`
- Email normalization (lowercased/trimmed)
- Nigerian phone normalization during hashing/verification
- Expiry window: `10 minutes`
- Resend cooldown: `30 seconds`
- Max resends: `3`
- Max verification attempts: `5`
- DB transaction wrapping for request and verify flows

## Database Notes

Startup currently runs `AutoMigrate` for:

- `models.User`
- `models.AuthSession`
- `models.RefreshToken`
- `modules/auth/otp.OTPModel`

Auth repository behavior to be aware of:

- Login lookups query the existing `wallet_users` table directly
- Session and refresh token models use explicit wallet table names (`wallet_auth_sessions`, `wallet_refresh_tokens`)
- `OTPModel` currently uses GORM's default table naming (no explicit `TableName()` override)

## Testing

Run tests:

```bash
go test ./...
```

Current automated coverage includes repository tests using `sqlmock` for the auth repository (`modules/auth/repository_test.go`).

## Known Gaps / Implementation Notes

- `POST /auth/refresh` handler is not finished even though service/repository refresh rotation logic exists
- `providers/sms/sms.go` currently sends a hardcoded OTP text (`123456`) instead of using the `message` argument passed into `Send`
- `internal/server/router.go` currently logs SMTP credentials at startup (should be removed or masked before production)

## Ownership Guidance

When editing, prefer these boundaries:

- Routing and dependency wiring: `internal/server/*`
- Runtime config and env mapping: `internal/config/*`
- DB connection/migrations: `internal/database/*`
- Notification interfaces shared across modules/providers: `internal/notify/*`
- Feature behavior: `modules/<feature>/*`
- External integrations: `providers/*`
- Shared DB models: `models/*`
- Email templates: `templates/*`

This keeps changes localized and reduces accidental cross-layer coupling.
