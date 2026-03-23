# Notification Service Logic Flow

This repository now supports a standalone notification service that can be deployed separately from the main mobile backend while still sharing the same database and JWT signing configuration.

## 1. Entry point and startup

- Binary: `cmd/notification-api/main.go`
- Server wiring: `internal/notificationserver/server.go`
- Router wiring: `internal/notificationserver/router.go`

Startup flow:

1. `cmd/notification-api/main.go` loads `.env` values and process environment variables.
2. `config.Load()` reads the notification-specific settings:
   - `NOTIFICATION_PORT`
   - `EXPO_PUSH_BASE_URL`
   - `EXPO_ACCESS_TOKEN`
   - `EXPO_PUSH_CHANNEL_ID`
   - `NOTIFICATION_INTERNAL_SECRET`
   - the shared `DB_URL` and `JWT_SECRET`
3. `notificationserver.NewRouter()` opens Postgres with retry logic.
4. `database.Migrate()` runs the shared schema migration, including `wallet_push_tokens`.
5. The notification router mounts:
   - public JWT-protected routes under `/api/v1/notifications`
   - internal HMAC-protected routes under `/internal/v1/notifications`
6. The process listens on `:<NOTIFICATION_PORT>`.

## 2. Storage model

Push tokens are stored in `wallet_push_tokens` through `models/push_token.go`.

Relevant columns:

- `id`
- `user_id`
- `device_id`
- `expo_push_token`
- `platform`
- `created_at`
- `updated_at`

Important constraints:

- one row per `user_id + device_id`
- one unique `expo_push_token`
- `user_id` references `wallet_users(id)` with `ON DELETE CASCADE`

That means:

- the same device updates its existing row instead of creating duplicates
- if a user is deleted, their push tokens are removed automatically
- a token cannot be silently shared across different user-device rows

## 3. Public client flow

### `POST /api/v1/notifications/token`

Handler: `modules/notification/handler.go`

Expected auth:

- `Authorization: Bearer <access_token>`

Request body:

```json
{
  "expo_push_token": "ExponentPushToken[xxxxxxxxxxxxxxxxxxxxxx]",
  "device_id": "device-123",
  "platform": "android"
}
```

Execution path:

1. `AuthGuard` validates the access token and sets the authenticated user ID in context.
2. `Handler.RegisterToken` binds the JSON body.
3. `Service.RegisterToken` validates:
   - user ID is present
   - device ID is present
   - platform is `ios` or `android`
   - the token looks like an Expo push token
4. `Repository.UpsertToken`:
   - removes any conflicting row already using the same `expo_push_token` for a different user/device pair
   - performs `ON CONFLICT (user_id, device_id) DO UPDATE`
5. The endpoint returns `200 {"message":"push token registered"}`.

Result:

- each mobile installation can keep its latest Expo token up to date
- reinstalling or refreshing the token does not create duplicate rows

### `DELETE /api/v1/notifications/token`

Expected auth:

- `Authorization: Bearer <access_token>`
- `X-Device-ID: <device_id>`

Execution path:

1. `AuthGuard` resolves the authenticated user.
2. `Handler.DeleteToken` reads `X-Device-ID`.
3. `Service.DeleteToken` validates user ID and device ID.
4. `Repository.DeleteTokenByUserAndDevice` deletes the row.
5. The endpoint returns `200 {"message":"push token deleted"}`.

Use this when a user logs out or disables push on a device.

## 4. Internal send flow

### `POST /internal/v1/notifications/send`

Expected auth:

- `X-Timestamp`
- `X-Signature`

The route is protected by `InternalHMACAuth` with `NOTIFICATION_INTERNAL_SECRET`.

Request body:

```json
{
  "user_id": "user-123",
  "title": "Loan Approved!",
  "body": "Your loan has been approved.",
  "data": {
    "screen": "/(loan)/details",
    "params": "{\"loanId\":\"abc123\"}"
  },
  "sound": "default",
  "channel_id": "default"
}
```

Execution path:

1. `Handler.SendNotification` binds the body.
2. `Service.SendToUserWithOptions` validates the request.
3. `Repository.ListTokensByUserID` loads all registered push tokens for the target user.
4. The service builds one `ExpoPushMessage` per token.
5. `providers/push/expo.go` sends the messages to Expo in batches of 100.
6. Expo returns push tickets.
7. For each ticket:
   - if it reports `DeviceNotRegistered`, the service deletes that token from `wallet_push_tokens`
   - other Expo errors are logged for investigation
8. The endpoint returns `200 {"message":"notification sent"}` if the send call itself succeeded.

Important behavior:

- this service is best-effort
- missing push tokens are not treated as an error
- tokens that Expo says are invalid are cleaned up automatically

## 5. Expo integration behavior

Outbound client: `providers/push/expo.go`

Requests:

- send endpoint: `POST https://exp.host/--/api/v2/push/send`
- receipts endpoint: `POST https://exp.host/--/api/v2/push/getReceipts`

Current implementation:

- sends JSON
- supports optional bearer auth with `EXPO_ACCESS_TOKEN`
- uses a 15 second HTTP timeout
- batches sends in chunks of 100

Current scope:

- send path is implemented and in use
- receipt lookup method exists but is not yet wired into a background job

## 6. Why this can be deployed separately

The notification service is isolated behind its own binary and port, but it still works with the rest of the system because it shares:

- the same Postgres database
- the same JWT secret used to authenticate mobile users
- the same user IDs stored in `wallet_users`

That gives you a clean split:

- `cmd/api` handles auth, device trust, BVN/NIN checks, and loan workflows
- `cmd/notification-api` handles device push-token storage and Expo dispatch

This lets you scale, deploy, and observe push delivery independently from the main mobile backend.
