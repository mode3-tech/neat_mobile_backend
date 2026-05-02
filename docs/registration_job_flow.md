# Registration Job Flow

This document describes the safe registration flow implemented in the auth module.

## Why the flow changed

The old flow created the provider wallet inside the HTTP request and only then tried to commit local database changes. That meant:

- local DB writes could roll back cleanly
- but the remote wallet/customer could already exist
- a retry could create another remote wallet

The new flow turns registration into a small saga backed by a durable job row.

## High-level behavior

`POST /api/v1/auth/register`

- validates the request
- consumes the verification records in a DB transaction
- stores a `wallet_registration_jobs` row with a sanitized registration snapshot
- returns `202 Accepted` with a `job_id`, a one-time `claim_token`, and `claim_expires_at`

`GET /api/v1/auth/register/:job_id/status`

- lets the client poll job progress
- returns `pending`, `processing`, `completed`, or `failed`
- returns `can_login=true` when registration is complete
- never returns access or refresh tokens

`POST /api/v1/auth/register/:job_id/claim`

- exchanges the one-time `claim_token` for access and refresh tokens
- only works after the job is `completed`
- requires the original `X-Device-ID`
- can only be used once

After the job is `completed`, the client should call the claim endpoint with the saved `claim_token`.
If the claim is no longer available, the client should use the normal login flow.

## State machine

Job statuses:

- `pending`: accepted locally and waiting to be processed
- `processing`: a worker has claimed the job
- `completed`: remote wallet creation and local finalization both succeeded
- `failed`: processing stopped with an error; the same registration request can be retried

## Request path

### 1. Deterministic idempotency

The register service computes a deterministic idempotency key from the normalized request payload.

The key is based on:

- normalized phone number
- email
- password
- transaction pin
- verification ids
- biometrics flag
- device payload

That means the same registration payload maps to the same job.

Effects:

- if the client retries the exact same request, the server finds the same job
- if the old job failed, the retry requeues it instead of consuming verification records again
- if the old job already completed, the server returns the completed job state
- if the old job has not been claimed yet, the retry rotates and returns a fresh `claim_token`

### 2. Local transaction

Inside one DB transaction, the service:

- checks for an existing job with the same idempotency key
- rejects a second open registration for the same phone
- loads and validates the phone/BVN/NIN/email verification rows
- checks for an existing user by phone and email
- validates password and transaction pin
- hashes password and pin
- generates a one-time registration `claim_token` and stores only its hash on the job
- marks verification rows as used
- stores a `wallet_registration_jobs` row containing:
  - generated `mobile_user_id`
  - generated `internal_wallet_id`
  - normalized phone
  - sanitized registration snapshot JSON

If this transaction fails, nothing is consumed locally.

## Worker path

Background processing is driven in two ways:

- an immediate async kick after a job is created or requeued
- a cron sweep every 5 seconds as a safety net

Workers claim jobs with `FOR UPDATE SKIP LOCKED`, so only one worker processes a given job at a time.

### 3. Claiming

Claiming does the following:

- resets stale `processing` jobs older than the lease timeout back to `pending`
- locks `pending` jobs
- marks claimed jobs as `processing`
- increments `attempts`

### 4. Provider wallet creation

When a worker processes a job:

- it decodes the registration snapshot
- if a stored `wallet_response_json` already exists, it reuses it
- on retry paths, it first looks up the remote wallet by `customer_id = mobile_user_id`
- otherwise it calls `GenerateWallet(...)`

Important details:

- the provider request now uses stable request data
- the old random BVN/phone/name/email overrides were removed
- provider metadata uses `customer_id = mobile_user_id`

If the provider succeeds, the worker stores the raw wallet response JSON on the job before local finalization.

If wallet creation fails but the provider may already have created the customer, the worker also falls back to the provider lookup endpoint:

- `GET /wallet/customer?customerId=<mobile_user_id>`

This lets retries recover the remote wallet/customer state without blindly creating another remote wallet.

### 5. Local finalization transaction

After a wallet response is available, the worker opens a new DB transaction and:

- creates `wallet_users`
- links the BVN row to the user
- creates `wallet_customer_wallets`
- binds the device
- marks the registration job `completed`

These local writes succeed or fail together.

After commit, the existing async CBA sync is triggered.

### 6. Session claim

Once the job is `completed`, the client exchanges the saved `claim_token` by calling:

`POST /api/v1/auth/register/:job_id/claim`

The claim step:

- verifies the supplied token against the stored hash
- checks that the job is completed and still unclaimed
- confirms the request is coming from the original `X-Device-ID`
- marks the claim as used in the same DB transaction that creates auth session rows
- returns the new access and refresh tokens

This keeps token delivery off the public status endpoint while still allowing immediate post-registration sign-in.

## Status endpoint behavior

`GET /api/v1/auth/register/:job_id/status`

Response shape:

- `job_id`
- `registration_status`
- `message`
- `can_login`
- `can_claim_session`
- `claim_expires_at` while an unclaimed session is still available
- `error` when the job is failed

Typical meanings:

- `pending`: job accepted locally
- `processing`: worker is running
- `completed`: registration is done; client should claim the saved registration session or log in normally
- `failed`: client can retry the same original register request

## Why this is safer

This flow protects the local system in a way the old one could not:

- verification records are consumed atomically with job creation
- local user/wallet/device writes happen in one finalization transaction
- repeated identical register requests reuse the same job
- provider success can survive a later local DB failure because the wallet response is persisted on the job
- access and refresh tokens are not exposed from the public status endpoint
- the automatic post-registration session can only be claimed once from the original device

## Remaining caveat

There is still one much narrower cross-system gap:

- provider wallet creation succeeds
- but the app crashes or loses DB access before saving `wallet_response_json`

If that happens, a later retry now tries the provider lookup endpoint using the stable `customer_id = mobile_user_id` before attempting another create.

The remaining risk is now mostly about provider-side visibility timing:

- wallet creation succeeds
- but the lookup endpoint does not yet return that customer on the immediate retry

In that case, repeated retries may still be needed until the provider lookup becomes consistent.

So the flow is now operationally recoverable through provider reconciliation, even though true distributed atomicity still depends on provider behavior.

## Placeholder provider fields

The app does not currently collect a registration address and email is optional.

For wallet creation, the worker uses:

- the real email if one was supplied
- otherwise a stable placeholder email derived from `mobile_user_id`
- a stable placeholder address

These values are deterministic, so retries do not drift.

## Files involved

- `modules/auth/service_register.go`
- `modules/auth/service_registration_jobs.go`
- `modules/auth/service_registration_claim.go`
- `modules/auth/repository_registration_jobs.go`
- `modules/auth/registration_job.go`
- `modules/auth/handler.go`
- `modules/auth/routes.go`
- `internal/server/router.go`
- `internal/database/database.go`
- `providers/providus/providus.go`
