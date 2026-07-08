# Registration contact verification (phone OR email)

## Why

Registration originally required a verified **phone** (`phone_verification_id`) — the phone on the user's BVN. Some users no longer have access to that phone but do have their email. The flow now accepts a **primary contact OTP that can be phone _or_ email**, and — when the primary OTP is email — an **alternate phone the user submits and verifies**, so the wallet still gets a reachable number. An optional **email** can also be captured for phone-first users.

Every contact that lands on the account is **OTP-verified**; no unverified contact is ever stored.

## Verification-record types

Records live in `wallet_verification_records` (`models.VerificationRecord`) and now carry a role-specific `type`:

| Record | `type` | Produced by |
|---|---|---|
| Primary signup contact (phone **or** email) | `otp` | OTP flow with `purpose = signup` |
| Submitted alternate phone | `phone` | OTP flow with `purpose = submitted_contact`, SMS channel |
| Submitted email | `email` | OTP flow with `purpose = submitted_contact`, email channel |
| BVN / NIN | `bvn` / `nin` | BVN/NIN validation |

An `otp` record still carries the actual contact in `verified_phone` **or** `verified_email` (populated by channel). The type is driven by **`Purpose`**, not channel — see `otp/service.go:newVerificationRecord` (`purpose == PurposeSignup → VerificationTypeOTP`, else channel → `phone`/`email`).

### Requesting a submitted-contact OTP
The signup OTP endpoints accept an optional `purpose` field (`signup` default, or `submitted_contact`). `RequestSMSOTPRequest` also accepts a `destination` so an OTP can be sent to a user-typed phone (not just a phone already on a verification record). Issue **and** verify must use the same `purpose`.

## Registration request (`RegisterationRequest`)

- `otp_verification_id` (**required**) — the primary contact OTP; must resolve to a `type = otp` record.
- `submitted_phone_verification_id` (optional) — **required when the OTP was email**; must resolve to a `type = phone` record.
- `email_verification_id` (optional) — a verified email for phone-first users; must resolve to a `type = email` record.
- `email` — **removed**. The account email is derived from verified records only.
- Plus BVN/NIN (`bvn_verification_id`, `nin_verification_id`) and their face checks, credentials, device, biometrics.

## Resolution rules (in `service_register.go`)

**Canonical phone** (`normalizedPhone`, always set):
- phone-OTP → the OTP record's `verified_phone`.
- email-OTP → the submitted phone record's `verified_phone` (submitted id is required, else `PHONE_OR_EMAIL_NOT_FOUND`).

**Account email** (`accountEmail`, optional):
- OTP-email if the OTP channel was email; else the `email_verification_id` record's `verified_email`; else empty.
- `IsEmailVerified = accountEmail != ""`.

**Flags:** `IsPhoneVerified` is always `true` (there is always a verified phone). `IsEmailVerified` reflects a real verified email.

## Validation invariants enforced

- **Type per slot:** `otp_verification_id → otp`, `submitted_phone_verification_id → phone`, `email_verification_id → email`, `bvn → bvn`, `nin → nin`. This blocks a BVN record (which carries a phone) from masquerading as the OTP contact, and blocks passing one record as both BVN and NIN (which would trivially satisfy the name/DOB cross-check).
- **Uniqueness:** one account per phone (`GetUserByPhone`) and per verified email (`GetUserByEmail`, only when present). Both keys are OTP-verified, so no squatting.
- **Consumption:** the OTP, BVN, NIN, submitted-phone (when used), and email (when used) records are all marked `used` inside the registration transaction.
- **Idempotency key** includes `otp_verification_id`, `submitted_phone_verification_id`, `email_verification_id`, the normalized phone and the verified email.

## Known gaps / follow-ups

- **Freshness / TTL.** `GetValidationRow` now filters `expires_at > now()`, so verification records are rejected once their **15-minute** TTL lapses. That TTL was designed for OTP codes and is likely **too short for a full multi-step registration** — a user who verifies BVN then spends >15 min on NIN + face + form will have the BVN record expire and registration will fail. Extending the verification-record TTL for the registration window is recommended before relying on this in production.
- **Login by email.** Users who registered email-first must be able to authenticate by email; ensure the login path accepts an email identifier.
- **Breaking API changes.** `otp_verification_id` replaced the old `phone_verification_id`; `email` was removed from the registration body; the new OTP `purpose`/`destination` fields were added. Coordinate with the client release.
- **Tests.** Recommended table tests: phone-OTP (no submitted phone); email-OTP + submitted phone; email-OTP **without** submitted phone → rejected; email-OTP + `email_verification_id`; a BVN or `phone` record in the `otp` slot → rejected; same id for BVN and NIN → rejected. Plus OTP-flow tests asserting `PurposeSignup → otp` and `PurposeSubmittedContact → phone/email`.
