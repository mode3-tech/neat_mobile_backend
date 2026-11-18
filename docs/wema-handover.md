# Wema Frontend Handover

This note describes the current Wema signup, account-upgrade, bank lookup/name-resolution, and bank-statement API flows. Swagger is served from `/openapi/doc.yaml` and `/openapi/doc.json`; these endpoints are documented there.

## Wema signup

Signup v2 endpoints are public and use the `/api/v2/auth` prefix. Do not send an auth token or `X-Device-ID` during signup; device details are required in the final registration body.

1. Request and verify the phone OTP:
   - `POST /api/v2/auth/otp/phone/request` with `{"phone_number":"..."}` returns `data.otp_id`.
   - `POST /api/v2/auth/otp/phone/verify` with `{"otp_id":"...","code":"..."}` returns `data.verification_id`. Keep this as `phone_verification_id`.
2. Request and verify the email OTP in the same way using `/api/v2/auth/otp/email/request` and `/api/v2/auth/otp/email/verify`. Keep its `data.verification_id` as `email_verification_id`.
3. Validate BVN with `POST /api/v2/auth/validate/bvn` using `bvn`, `dob`, and `email`. For Wema, `data.requires_otp` is false; there is no provider OTP step.
4. Verify the BVN with a face image using `POST /api/v2/auth/validate/bvn-with-face`:

   ```json
   {
     "verification_id": "BVN_VERIFICATION_ID",
     "bvn": "BVN",
     "image": "IMAGE_DATA"
   }
   ```

   Save `data.bvn_w_face_verification_id` for registration. Wema registration requires a matched face check tied to the BVN.
5. Submit `POST /api/v2/auth/register` with the verified contact IDs, applicant details, BVN/NIN, password/PIN, device object, and `bvn_w_face_verification_id`. Set `supports_pending: true`.

   Account creation can finish immediately (`data.status: "completed"`, tokens returned) or be asynchronous (`data.status: "pending"`, HTTP 202). For pending results, securely retain both `data.registration_id` and `data.claim_token`, then poll `POST /api/v2/auth/register/status` with:

   ```json
   {
     "registration_id": "REGISTRATION_ID",
     "claim_token": "CLAIM_TOKEN"
   }
   ```

   Keep polling while `data.status` is `pending`. On `completed`, use the returned access and refresh tokens. If `login_required` is true, direct the user to log in. A failed or expired status should be shown as an error; do not retry registration blindly.

## Account upgrades

These are authenticated v1 endpoints and require both `Authorization: Bearer <access_token>` and `X-Device-ID`.

- **Tier 2:** `POST /api/v1/tier/validate/nin/face` with `{"image":"IMAGE_DATA"}` validates the signed-in user's stored NIN/DOB against the face. For Wema, a successful match also submits the bank-side Tier 2 upgrade; there is no separate Tier 2 upgrade route. If ALAT rejects the upgrade, the request can return an error even though the local face/NIN verification passed. Refresh `GET /api/v1/tier/kyc-status` before deciding what the user's bank-side tier is.
- **Tier 3:** `POST /api/v1/tier/upgrade/tier3` with a residential address:

  ```json
  {
    "residential_address": {
      "building_number": "12",
      "street": "Broad Street",
      "city": "Lagos",
      "state": "Lagos",
      "lga": "Lagos Island",
      "country": "Nigeria",
      "full_address": "12 Broad Street, Lagos",
      "apartment": "",
      "town": "",
      "lcda": "",
      "landmark": "",
      "additional_information": "",
      "postal_code": ""
    }
  }
  ```

  Building number, street, city, state, LGA, country, and full address are required. A successful response means ALAT accepted the request, not necessarily that KYC review is complete.
- **Check bank status:** `GET /api/v1/tier/kyc-status` returns ALAT's `data` fields such as `accountTier`, `accountStatus`, `restrictionStatus`, and `addressVerificationStatus`. Treat the values as provider-defined strings.

## Transfer bank lookup and name resolution

The Wema-specific v2 lookup endpoints require `Authorization: Bearer <access_token>`, `X-Device-ID`, and a Wema/ALAT wallet:

- `GET /api/v2/wallet/banks` returns ALAT's supported banks in `data` as `{ "code": "...", "name": "..." }` items.
- `GET /api/v2/wallet/bank/details?account_number=...&bank_code=...` performs ALAT account-name enquiry. Use a bank code returned by the first endpoint; the response's `data` contains `bankCode`, `accountName`, and `accountNumber`.

These v2 endpoints reject accounts that do not have a Wema wallet. The existing transfer operation remains `POST /api/v1/wallet/transfer`; this change only adds Wema bank listing and recipient-name resolution in v2.

## Account statement

The v2 statement URLs are provider-neutral but work only for a Wema/ALAT wallet. They require the same bearer token and `X-Device-ID` as account upgrades.

- `POST /api/v2/account/statement/` with `{"date_from":"YYYY-MM-DD","date_to":"YYYY-MM-DD"}` returns `data.statement_id`.
- `GET /api/v2/account/statement/{statement_id}` returns a JSON envelope with `data` as an array of transactions. Preserve the statement ID as an opaque value and use it with the same signed-in user; it is user-bound.

The transaction fields are listed in Swagger. The result is JSON, not a PDF/XLSX. This differs from the older `/api/v1/account/statement` endpoint, which generates a file from the app's local ledger and may not include all bank-side activity. ALAT does not document statement readiness timing; if the GET is not ready, retry it with the same ID.

## Frontend notes

- Never put ALAT API keys, `ALAT_SECURITY_SECRET`, or webhook configuration in the app. Those are backend-only settings.
- The backend has not yet been verified against production ALAT credentials and a production Wema account. Confirm the ALAT response timing and account-upgrade status behavior before assuming synchronous completion.
- VAS outcomes can remain pending until an ALAT direct-account VAS status flow is confirmed; Wema VAS cashback is not supported.
