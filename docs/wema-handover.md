# Wema Frontend Handover

This note describes the current Wema signup, account-upgrade, bank lookup/name-resolution, transfer (including ALAT-to-ALAT), and bank-statement API flows. Swagger is served from `/openapi/doc.yaml` and `/openapi/doc.json`; these endpoints are documented there.

## Wema signup

Signup v2 endpoints are public and use the `/api/v2/auth` prefix. Do not send an auth token or `X-Device-ID` during signup; device details are required in the final registration body.

1. Request and verify the phone OTP:
   - `POST /api/v2/auth/otp/phone/request` with `{"phone_number":"..."}` returns `data.otp_id`.
   - `POST /api/v2/auth/otp/phone/verify` with `{"otp_id":"...","code":"..."}` returns `data.verification_id`. Keep this as `phone_verification_id`.
2. Request and verify the email OTP in the same way using `/api/v2/auth/otp/email/request` and `/api/v2/auth/otp/email/verify`. Keep its `data.verification_id` as `email_verification_id`.
3. Validate BVN with `POST /api/v2/auth/validate/bvn` using `bvn`, `dob`, and `email`. For Wema, `data.requires_otp` is false; there is no provider OTP step.
4. Do Wema's face check: `POST /api/v2/auth/wema/face-url` returns the URL of Wema's face web app, which the app opens in a webview. Wema reports the result (including its correlation ID) to the backend's callback, so the app never handles the correlation ID. Poll `POST /api/v2/auth/wema/face-status` (same body) until `data.status` is `verified` (or `failed`, with Wema's reason in `data.message`). This replaces `POST /api/v2/auth/validate/bvn-with-face` and `bvn_w_face_verification_id` for Wema. See [wema-face-verification-frontend.md](wema-face-verification-frontend.md).
5. Submit `POST /api/v2/auth/register` with the verified contact IDs, applicant details, BVN/NIN, password/PIN and device object (no `bvn_w_face_verification_id`). `supports_pending` is not needed for Wema.

   Account creation can finish immediately (`data.status: "completed"`, tokens returned) or be asynchronous (`data.status: "pending"`, HTTP 202, always returned for Wema when the account isn't ready, with Wema's own text in `message`; show a pending screen with it). For pending results, securely retain both `data.registration_id` and `data.claim_token`, then poll `POST /api/v2/auth/register/status` with:

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

## ALAT-to-ALAT (Wema-to-Wema) transfers

There is no separate intra-bank endpoint. A transfer to another Wema/ALAT account uses the same flow as an interbank transfer, with Wema's bank code ` ` (`WEMA BANK`) as the destination bank. ALAT receives the same `ProcessClientTransfer` call either way and routes it internally when the destination is Wema.

1. **Pick the bank.** If the `GET /api/v2/wallet/banks` list contains a Wema entry, use it. It is not yet confirmed whether ALAT includes Wema in that list. If it doesn't, add a pinned "Wema Bank / ALAT" option with code `000017`. Don't add a second Wema entry if the list already has one.
2. **Resolve the name.** Call `GET /api/v2/wallet/bank/details?account_number=...&bank_code=000017`. Show the returned `data.accountName` and let the user confirm it. Don't let the user type the recipient name.
3. **Send the transfer.** Call `POST /api/v1/wallet/transfer` with the same headers as other wallet calls:

   ```json
   {
     "amount": 1000,
     "sort_code": "000017",
     "account_number": "0123456789",
     "account_name": "NAME FROM STEP 2",
     "narration": "optional, max 255 chars",
     "transaction_pin": "1234"
   }
   ```

   `amount` is in naira and must be greater than 50. `account_name` should be the exact `accountName` from step 2. The backend checks the live ALAT balance before debiting and returns an insufficient-balance error if needed.
4. **Treat a successful response as pending, not completed.** ALAT transfers are asynchronous. ALAT returns "pending" and only moves the money after it calls the backend's authentication callback. A background status sweep also checks unfinished transfers. A 2xx response means ALAT accepted the request. Show a "processing" state, not "successful".
5. **Get the final status.** Store `data.reference`, which is the backend transaction ID (it also appears as `transactionReference`). Poll `GET /api/v1/transaction/{reference}` with backoff, or refresh the transactions list, until the status is final. Don't resend the transfer while it is pending: every request creates a new debit, so a retry can send the money twice.

Other notes:

- Self-transfers (to the user's own ALAT account number) aren't blocked by the backend. Block them in the UI.
- The Wema adapter reports `charges` and `vat` as 0. Don't show a fee for ALAT-to-ALAT transfers unless the backend starts returning one.
- Beneficiaries saved with `bank_code: "000017"` work the same way. Run the name enquiry again before sending if the saved name could be stale.

## Account statement

The v2 statement URLs are provider-neutral but work only for a Wema/ALAT wallet. They require the same bearer token and `X-Device-ID` as account upgrades.

- `POST /api/v2/account/statement/` with `{"date_from":"YYYY-MM-DD","date_to":"YYYY-MM-DD"}` returns `data.statement_id`.
- `GET /api/v2/account/statement/{statement_id}` returns a JSON envelope with `data` as an array of transactions. Preserve the statement ID as an opaque value and use it with the same signed-in user; it is user-bound.

The transaction fields are listed in Swagger. The result is JSON, not a PDF/XLSX. This differs from the older `/api/v1/account/statement` endpoint, which generates a file from the app's local ledger and may not include all bank-side activity. ALAT does not document statement readiness timing; if the GET is not ready, retry it with the same ID.

## Frontend notes

- Never put ALAT API keys, `ALAT_SECURITY_SECRET`, or webhook configuration in the app. Those are backend-only settings.
- The backend has not yet been verified against production ALAT credentials and a production Wema account. Confirm the ALAT response timing and account-upgrade status behavior before assuming synchronous completion.
- VAS outcomes can remain pending until an ALAT direct-account VAS status flow is confirmed; Wema VAS cashback is not supported.
