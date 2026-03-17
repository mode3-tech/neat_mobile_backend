# Neat Mobile App Backend

Go HTTP API backend for authentication, OTP verification, device trust, identity validation, password reset, and loan product workflows. The service uses Gin, PostgreSQL, GORM, and JWT.

## Base URLs

- API: `http://localhost:<PORT>/api/v1`
- Swagger UI: `http://localhost:<PORT>/swagger/index.html`
- OpenAPI JSON: `http://localhost:<PORT>/openapi/doc.json`
- OpenAPI YAML: `http://localhost:<PORT>/openapi/doc.yaml`

## Implemented API Surface

### Auth and Verification

- `POST /auth/otp/request`
- `POST /auth/otp/verify`
- `POST /auth/validate-bvn`
- `POST /auth/validate-nin`
- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/verify-device`
- `POST /auth/verify-new-device`
- `POST /auth/refresh`
- `POST /auth/logout`
- `POST /auth/forgot-password`
- `POST /auth/reset-password`

### Loan

- `GET /loan/`
- `POST /loan/apply`

## Auth Flow

1. Verify contact points with `/auth/otp/request` and `/auth/otp/verify`.
2. Validate BVN with `/auth/validate-bvn`.
3. Validate NIN with `/auth/validate-nin` using the `bvn_validation_id` returned from the BVN step.
4. Register with:
   - verification IDs
   - `password` and `confirm_password`
   - `transaction_pin` and `confirm_transaction_pin`
   - `is_biometrics_enabled`
   - the initial `device` payload
5. Log in with `phone`, `password`, and the `X-Device-ID` header.

Login behavior:

- Trusted device: `/auth/login` returns `challenge_required`, then `/auth/verify-device` completes login with `challenge`, `signature`, and `device_id`.
- New or untrusted device: `/auth/login` returns `new_device_detected`, sends an SMS OTP, and returns a `session_token` for `/auth/verify-new-device`.
- `/auth/verify-new-device` expects `session_token`, `otp`, and a full `device` payload. On success the device is trusted and access and refresh tokens are issued.
- `/auth/forgot-password` and `/auth/reset-password` also require `X-Device-ID`.

Device challenge signatures use `ecdsa-p256-sha256` over `SHA-256(challenge)`.

## Loan Flow

- Loan endpoints require `Authorization: Bearer <access_token>`.
- `GET /loan/` returns the current loan products from `wallet_loan_products`.
- `POST /loan/apply` validates the authenticated user, the selected product, business input, and core-banking loan checks before creating a pending application.
- Supported product codes in the current service are `BUSINESS-WK`, `SPECIAL-WK`, `SME-WK`, `SALARY-MTH`, `INDIVIDUAL-WK`, and `GROUP-WK`.
- `business_start_date` must be in `YYYY-MM` format.
- Listing products works without core-banking connectivity, but `/loan/apply` depends on `CBA_INTERNAL_URL` and `CBA_INTERNAL_KEY`. If those are not configured, applications fail with service-unavailable errors.

## Runtime Notes

- Startup loads configuration, connects to Postgres with retry, runs migrations, and mounts the API plus Swagger assets.
- Auto-migrations cover users, auth sessions, refresh tokens, verification records, pending device sessions, OTP rows, user devices, device challenges, loan products, loan product rules, and loan applications.
- Login is rate-limited with the `LOGIN_RATE_LIMIT_*` configuration.
- OTP flows use resend throttling and attempt limits.
- Auth error responses include `request_id` values from the request middleware.

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

Identity and core adapters:

- `TENDAR_APIKEY`
- `PREMBLY_APIKEY`
- `CBA_INTERNAL_URL`
- `CBA_INTERNAL_KEY`

Login rate limiter:

- `LOGIN_RATE_LIMIT_IP_MAX_ATTEMPTS`
- `LOGIN_RATE_LIMIT_EMAIL_MAX_ATTEMPTS`
- `LOGIN_RATE_LIMIT_WINDOW_MINUTES`
- `LOGIN_RATE_LIMIT_BLOCK_MINUTES`

Dev signer:

- `DEV_SIGNER_KEYS_JSON`
- `DEV_SIGNER_KEYS_FILE`
- `DEV_SIGNER_DEVICE_ID`
- `DEV_SIGNER_PRIVATE_KEY`
- `DEV_SIGNER_ADDR`

## Local Run

```bash
go mod download
go run ./cmd/api
```

## Seed Loan Data

Seed the loan products:

```bash
go run ./cmd/seed-loan-products
```

Seed the loan product rules:

```bash
go run ./cmd/seed-loan-products-rules
```

Optional flags:

- `-dir` to override the default seed directory
- `-dry-run` to validate files without writing rows

## Dev Signer

Use `cmd/devsigner` when testing `/auth/verify-device` without a mobile client.

Run it with:

```bash
go run ./cmd/devsigner
```

Supported configuration:

- `DEV_SIGNER_KEYS_JSON`: JSON object like `{ "sim-device-1": "<private-key>" }`
- `DEV_SIGNER_KEYS_FILE`: path to a JSON file with the same shape
- `DEV_SIGNER_DEVICE_ID` and `DEV_SIGNER_PRIVATE_KEY`: single-device shortcut
- `DEV_SIGNER_ADDR`: listen address, default `:9090`

Endpoints:

- `GET http://localhost:9090/health`
- `POST http://localhost:9090/sign`

The signer expects ECDSA P-256 private keys in PEM, DER, PKCS8, raw 32-byte scalar, base64, base64url, or hex-encoded forms.

## Project Layout

```text
neat_mobile_app_backend/
|-- cmd/
|   |-- api/
|   |-- devsigner/
|   |-- seed-loan-products/
|   `-- seed-loan-products-rules/
|-- docs/
|-- internal/
|-- models/
|-- modules/
|   |-- auth/
|   |-- device/
|   `-- loanproduct/
|-- providers/
|-- seed/
`-- templates/
```
