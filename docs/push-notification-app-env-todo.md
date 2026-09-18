# Mobile TODO: send `app_env` when registering push tokens

## Why
Backend now dedupes push notifications per `(user_id, app_env)` so a user with
multiple builds of the app installed on one phone (e.g. production + staging + dev)
doesn't get one push notification per build. Each build currently registers its
own `device_id` and Expo push token, and the backend has no way to tell "3 builds
on 1 phone" apart from "3 real separate devices" unless the app tells it which
build/environment it's running.

The backend change alone does nothing until the app starts sending this field —
until then, all builds will keep getting notified separately as before (dedup
only kicks in for tokens that report an `app_env`).

## What to do
When calling `POST /api/v1/notifications/token`, add an `app_env` field to the
request body identifying the build:

```json
{
  "expo_push_token": "ExponentPushToken[...]",
  "device_id": "...",
  "platform": "ios",
  "app_env": "production"
}
```

- Use a constant baked into each build's config (e.g. `app.config.js` /
  build-time env var) — something like `production`, `staging`, or `dev`.
- It must be the **same value across app restarts/reinstalls of the same build**,
  and **different per build variant**, since dedup keeps only the most recently
  updated token per `(user, app_env)`.
- The field is optional on the backend for now (existing app versions without it
  keep working exactly as today, un-deduped) — but nothing improves for users
  with multiple builds until this ships.

## Heads up / trade-off
If a user is ever expected to have the *same* build genuinely installed on two
different physical phones and wants both notified, this dedup will only notify
the most recently active one. Flag if that's a real use case — the current
default assumes it isn't.
