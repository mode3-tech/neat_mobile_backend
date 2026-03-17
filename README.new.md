# Neat Mobile App Backend

Go HTTP API backend built with Gin, GORM, PostgreSQL, and JWT for:

- account registration and login
- OTP issuance and verification
- device-aware authentication
- BVN and NIN validation
- loan product catalog access and loan applications

## API Surface

Base API URL: `http://localhost:<PORT>/api/v1`

Swagger endpoints served by the main API:

- UI: `http://localhost:<PORT>/swagger/index.html`
- JSON: `http://localhost:<PORT>/openapi/doc.json`
- YAML: `http://localhost:<PORT>/openapi/doc.yaml`

Main routes:

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/verify-device`
- `POST /auth/verify-new-device`
- `POST /auth/refresh`
- `POST /auth/logout`
- `POST /auth/validate-bvn`
- `POST /auth/validate-nin`
- `POST /auth/forgot-password`
- `POST /auth/reset-password`
- `POST /auth/otp/request`
- `POST /auth/otp/verify`
- `GET /loan/`
- `POST /loan/apply`

## Current Behavior

Registration expects prior verification ids plus initial device metadata. The current request body includes:

- `phone_number`
- `password` and `confirm_password`
- `transaction_pin` and `confirm_transaction_pin`
- `bvn_verification_id`
- `nin_verification_id`
- `phone_verification_id`
- optional `email` and `email_verification_id`
- `is_biometrics_enabled`
- `device`

Login is device-aware:

- trusted devices return `{"status":"challenge_required","challenge":"..."}`
- new or untrusted devices send an SMS OTP and return `{"status":"new_device_detected","session_token":"..."}`

Use `POST /auth/verify-device` for trusted devices and `POST /auth/verify-new-device` for the SMS OTP flow. `POST /auth/verify-new-device` also stores the submitted device public key and trusts the device for later challenge logins.

Password reset requires a previously bound device through the `X-Device-ID` header. Loan routes require `Authorization: Bearer <access_token>`.

## Loan Module

`GET /loan/` returns the seeded loan product catalog.

`POST /loan/apply` creates a loan application and returns an estimated repayment summary. The handler validates:

- selected loan product
- business age and amount formatting
- user age and verification state
- loan product rule limits
- core banking customer and loan checks

`/loan/apply` depends on the internal CBA integration. If `CBA_INTERNAL_URL` and `CBA_INTERNAL_KEY` are not configured, application requests will fail even though catalog listing still works.

## Environment Variables

Runtime config is loaded from `internal/config/config.go`. The server currently expects these variable names:

Core:

- `PORT`
- `DB_URL`
- `JWT_SECRET`
- `PEPPER`

SMS and email:

- `TERMII_APIKEY`
- `TERMII_SENDERID`
- `SMTP_HOST`
- `SMTP_PORT`
- `SMTP_USER`
- `SMTP_PASS`

Identity and internal services:

- `TENDAR_APIKEY`
- `PREMBLY_APIKEY`
- `CBA_INTERNAL_URL`
- `CBA_INTERNAL_KEY`

Login rate limiter:

- `LOGIN_RATE_LIMIT_IP_MAX_ATTEMPTS`
- `LOGIN_RATE_LIMIT_EMAIL_MAX_ATTEMPTS`
- `LOGIN_RATE_LIMIT_WINDOW_MINUTES`
- `LOGIN_RATE_LIMIT_BLOCK_MINUTES`

## Local Run

```bash
go mod download
go run ./cmd/api
```

On startup the app connects to Postgres, runs migrations, and serves the API plus Swagger assets.

Current migrations include:

- auth session and refresh token tables
- verification records
- pending device sessions
- OTPs
- trusted devices and device challenges
- loan products, loan product rules, and loan applications

## Seed Loan Data

Seed loan products:

```bash
go run ./cmd/seed-loan-products
```

Seed loan rules:

```bash
go run ./cmd/seed-loan-products-rules
```

Dry-run validation is also supported:

```bash
go run ./cmd/seed-loan-products --dry-run
go run ./cmd/seed-loan-products-rules --dry-run
```

## Dev Signer

`cmd/devsigner` is a separate helper service for local testing of `/auth/verify-device`.

Base URL: `http://localhost:9090`

Routes:

- `GET /health`
- `POST /sign`

Environment:

- `DEV_SIGNER_KEYS_JSON`
- `DEV_SIGNER_KEYS_FILE`
- `DEV_SIGNER_DEVICE_ID`
- `DEV_SIGNER_PRIVATE_KEY`
- `DEV_SIGNER_ADDR`

Supported key formats:

- ECDSA P-256 PEM
- ECDSA P-256 PKCS8
- raw 32-byte scalar encoded as base64, base64url, or hex

Example:

```bash
go run ./cmd/devsigner
```

```http
POST http://localhost:9090/sign
Content-Type: application/json

{
  "device_id": "sim-device-1",
  "challenge": "challenge-from-login"
}
```

Response:

```json
{
  "signature": "base64url-signature",
  "algorithm": "ecdsa-p256-sha256"
}
```

## Notes

- `POST /auth/validate-bvn` currently forces Tendar-backed validation.
- `POST /auth/validate-nin` expects `bvn_validation_id`, not `bvn_verification_id`.
- `POST /auth/otp/request` and `POST /auth/otp/verify` are generic OTP endpoints. The new-device login flow uses the dedicated login + verify-new-device flow instead.
