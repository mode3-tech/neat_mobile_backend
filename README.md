# Neat Mobile App Backend

Go HTTP API backend for authentication, OTP verification, device trust, identity validation, password reset, and loan product workflows. The service uses Gin, PostgreSQL, GORM, and JWT.

## Base URLs

- API: `http://localhost:<PORT>/api/v1`
- Internal CBA: `http://localhost:<PORT>/internal/v1`
- Notification API: `http://localhost:<NOTIFICATION_PORT>/api/v1`
- Notification Internal: `http://localhost:<NOTIFICATION_PORT>/internal/v1`
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

- `GET /loan`
- `POST /loan/apply`
- `GET /loan/loans`
- `GET /loan/repayment-schedule?loan_id=<loan_id>`

### Notification Service

- `GET /api/v1/notifications?page=<page>&page_size=<page_size>`
- `GET /api/v1/notifications/unread-count`
- `PATCH /api/v1/notifications/:id/read`
- `PATCH /api/v1/notifications/read-all`
- `POST /api/v1/notifications/token`
- `DELETE /api/v1/notifications/token`
- `POST /internal/v1/notifications/send`

### Internal CBA

- `GET /internal/v1/cba/loan-applications?user_id=<mobile_user_id>`
- `GET /internal/v1/cba/loan-applications/embryo`
- `GET /internal/v1/cba/loan-applications/:application_ref`
- `PATCH /internal/v1/cba/loan-applications/:application_ref/status`
- `GET /internal/v1/cba/customers/bvn-record?user_id=<mobile_user_id>`
- `POST /internal/v1/cba/customers/link-by-bvn`
- `PATCH /internal/v1/cba/customers/:customer_id/status`

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

- `GET /loan` returns the current loan products from `wallet_loan_products`.
- `POST /loan/apply` validates the selected product, business input, `transaction_pin`, user verification state, and core-banking loan checks before creating an `embryo` application. The response includes `application_ref`, `loan_status`, and the estimated repayment `summary`.
- `GET /loan/loans` returns the authenticated user's core-banking loan list.
- `GET /loan/repayment-schedule?loan_id=<loan_id>` returns the repayment schedule for a specific core-banking loan id.
- Supported product codes in the current service are `BUSINESS-WK`, `SPECIAL-WK`, `SME-WK`, `SALARY-MTH`, `INDIVIDUAL-WK`, and `GROUP-WK`.
- `business_start_date` must be in `YYYY-MM` format.
- In the current router, `GET /loan/loans` and `GET /loan/repayment-schedule` are mounted with `authGuard`.
- Listing products works without core-banking connectivity, but `/loan/apply`, `/loan/loans`, and `/loan/repayment-schedule` depend on `CBA_INTERNAL_URL` and `CBA_INTERNAL_KEY`. If those are not configured, requests fail with service-unavailable errors.

## Internal CBA Flow

- Internal CBA endpoints are mounted under `/internal/v1/cba`.
- `GET /internal/v1/cba/loan-applications?user_id=<mobile_user_id>` returns the most recent loan application for that user where either the loan status is `embryo` or the customer status is `embryo`. The response envelope keeps `count` and `applications`, but returns at most one application.
- `GET /internal/v1/cba/loan-applications/embryo?page=<page>&limit=<limit>` returns applicant summaries for rows where either the loan status is `embryo` or the customer status is `embryo`. The response now includes `page`, `limit`, `total`, and `count`.
- `GET /internal/v1/cba/loan-applications/:application_ref` returns one loan application by local application reference.
- `GET /internal/v1/cba/customers/bvn-record?user_id=<mobile_user_id>` returns the linked BVN record for that wallet user together with the matched `application_ref`.
- `POST /internal/v1/cba/customers/link-by-bvn` links local wallet users to a supplied core customer id by BVN.
- `PATCH /internal/v1/cba/customers/:customer_id/status` updates wallet customer status for the supplied core customer id. Allowed values are `embryo`, `pending`, and `approved`.
- `PATCH /internal/v1/cba/loan-applications/:application_ref/status` updates local loan application status. Allowed values are `approved`, `decline`, and `active`; `active` requires `core_loan_id`.
- Customer-status and loan-status callbacks are idempotent by `event_id`. Replayed callbacks are ignored after the first successful insert into the event log.
- Internal requests must include `X-Timestamp` and `X-Signature`.
- `X-Timestamp` must be a fresh RFC3339 timestamp within five minutes of server time.
- `X-Signature` must be a lowercase hex HMAC-SHA256 of:
  `METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + SHA256_HEX_OF_BODY`
- The HMAC key is `CBA_WEBHOOK_SECRET`.
- For `GET` requests, the request body is empty, so the body hash is the SHA-256 of empty bytes.

## Notification Service Flow

- The standalone notification binary lives at `cmd/notification-api`.
- Startup loads environment variables, connects to the same Postgres instance, runs the shared migrations, then mounts only notification routes.
- `GET /api/v1/notifications` is JWT-protected and returns paginated in-app notification history ordered newest first.
- `GET /api/v1/notifications/unread-count` is JWT-protected and returns the unread badge count for the authenticated user.
- `PATCH /api/v1/notifications/:id/read` is JWT-protected and marks one notification as read for the authenticated user.
- `PATCH /api/v1/notifications/read-all` is JWT-protected and marks all unread notifications as read for the authenticated user.
- `POST /api/v1/notifications/token` is JWT-protected and upserts one `wallet_push_tokens` row per `user_id + device_id`.
- `DELETE /api/v1/notifications/token` is JWT-protected and removes the token for the authenticated user and `X-Device-ID`.
- `POST /internal/v1/notifications/send` is HMAC-protected and is intended for trusted internal callers, not mobile clients.
- The send flow creates a `wallet_notifications` history row first, then loads all push tokens for the target user, builds Expo push messages, sends them in batches of up to 100, persists accepted Expo ticket ids in `wallet_notification_tickets`, and removes tokens when Expo reports `DeviceNotRegistered`.
- The notification service uses `NOTIFICATION_INTERNAL_SECRET` for its internal HMAC route, separate from `CBA_WEBHOOK_SECRET`.
- End-to-end flow details are documented in [docs/notification-service-flow.md](docs/notification-service-flow.md).

## Runtime Notes

- `cmd/api` starts the main auth and loan backend.
- `cmd/notification-api` starts the standalone notification backend.
- Both services load configuration, connect to Postgres with retry, and run the shared migrations before serving traffic.
- Auto-migrations cover users, BVN records, push tokens, notifications, notification tickets, auth sessions, refresh tokens, verification records, pending device sessions, OTP rows, user devices, device challenges, loan products, loan product rules, loan applications, loan application status events, and customer status events.
- `wallet_push_tokens.user_id` is constrained to `wallet_users(id)` with `ON DELETE CASCADE`.
- `wallet_notifications.user_id` is constrained to `wallet_users(id)` with `ON DELETE CASCADE`.
- Login is rate-limited with the `LOGIN_RATE_LIMIT_*` configuration.
- OTP flows use resend throttling and attempt limits.
- Auth error responses include `request_id` values from the request middleware.

## Environment Variables

Core:

- `PORT`
- `NOTIFICATION_PORT`
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
- `CBA_WEBHOOK_SECRET`

Push notifications:

- `EXPO_PUSH_BASE_URL`
- `EXPO_ACCESS_TOKEN`
- `EXPO_PUSH_CHANNEL_ID`
- `NOTIFICATION_INTERNAL_SECRET`

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

Run the standalone notification service:

```bash
go run ./cmd/notification-api
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
|   |-- notification-api/
|   |-- seed-loan-products/
|   `-- seed-loan-products-rules/
|-- docs/
|-- internal/
|   `-- notificationserver/
|-- models/
|-- modules/
|   |-- auth/
|   |-- device/
|   |-- loanproduct/
|   `-- notification/
|-- providers/
|   `-- push/
|-- seed/
`-- templates/
```
