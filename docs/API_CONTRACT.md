# API Contract

> 🌐 **English** · [Монгол](API_CONTRACT_MN.md)

REST API reference for the **Gerege Backend Template v27**. The live,
auto-generated spec is served at `GET /swagger/` (source: `docs/swagger.json`).

> **Origin.** Derived from the open-source
> [snykk/go-rest-boilerplate](https://github.com/snykk/go-rest-boilerplate)
> (MIT, by Najib Fikri); HTTP layer ported **Gin → chi (net/http)**, data layer
> **sqlx → pgx (pgxpool)**. See [ARCHITECTURE.md](./ARCHITECTURE.md#credits--license).

## Conventions

- **Base URL:** `http://localhost:8080/api/v1`
- **Content-Type:** `application/json`
- **Auth:** protected endpoints require `Authorization: Bearer <access_token>`
- **Rate limit:** `/auth/*` is capped at ~5 requests/minute per IP; `/ai/*` at ~20/minute (429 on excess)
- **Body cap:** `/auth/*` bodies are limited to 4 KiB; everything else to 1 MiB

### Response envelope

Every response uses one envelope:

```json
{
  "status": true,
  "message": "human-readable summary",
  "data": { },
  "request_id": "b1d2…"
}
```

- `status` — `true` on success, `false` on error
- `data` — present on success (omitted/null on error)
- `request_id` — correlation id (also echoed in the `X-Request-ID` header)

### Status codes

| Code | Meaning | When |
|------|---------|------|
| 200 | OK | Successful read / action |
| 201 | Created | Resource created (register) |
| 400 | Bad Request | Malformed body |
| 401 | Unauthorized | Missing/invalid/expired token, wrong credentials |
| 403 | Forbidden | Locked out (e.g. OTP / login brute-force) |
| 404 | Not Found | Resource does not exist |
| 409 | Conflict | Duplicate username/email |
| 422 | Unprocessable Entity | Validation failed (see below) |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Unexpected failure (cause logged, generic message returned) |

### Validation error (422)

Field-level detail is returned under `data.errors`, which is an **array** of
`{ field, tag, message }` objects. `field` is the JSON tag name (e.g.
`new_password`, `email`):

```json
{
  "status": false,
  "message": "validation failed",
  "data": { "errors": [ { "field": "new_password", "tag": "min", "message": "must be at least 12 characters long" } ] },
  "request_id": "b1d2…"
}
```

The `strongpassword` rule requires mixed case, a digit, and a special character.

---

## Authentication

### POST `/auth/register`
Register a new account (regular user role).

**Request**
```json
{ "last_name": "Бат", "first_name": "Дорж", "last_name_en": "Bat", "first_name_en": "Dorj",
  "username": "johndoe", "email": "john@example.com", "password": "Str0ng!Passw0rd" }
```
| Field | Rules |
|-------|-------|
| `last_name` / `first_name` | required, 1–50 chars (Mongolian) |
| `last_name_en` / `first_name_en` | optional, ≤ 50 (Latin) |
| `username` | required, 3–25 chars |
| `email` | required, valid email, ≤ 50 |
| `password` | required, 12–72, strongpassword |

**Response `201`**
```json
{ "status": true, "message": "registration user success", "data": { "user": { "id": "…", "username": "johndoe", "email": "john@example.com", "role_id": 2, "created_at": "…", "updated_at": null } }, "request_id": "…" }
```
Errors: `409` duplicate username/email, `422` validation.

### POST `/auth/login`
Authenticate and receive an access + refresh token pair. Wrong password and
unknown email take the same wall-clock time (timing-attack mitigation).

**Request**
```json
{ "email": "john@example.com", "password": "Str0ng!Passw0rd" }
```

**Response `200`**
```json
{ "status": true, "message": "login success", "data": {
  "id": "…", "username": "johndoe", "email": "john@example.com", "role_id": 2,
  "token": "<access_jwt>", "refresh_token": "<refresh_jwt>",
  "created_at": "…", "updated_at": null }, "request_id": "…" }
```
Errors: `401` bad credentials, `403` locked out after repeated failures, `422` validation.

### POST `/auth/send-otp`
Send a one-time code to the email (used to activate an account).

**Request** `{ "email": "john@example.com" }`
**Response `200`** — message `"otp code has been send to john@example.com"`, `data: null`.

### POST `/auth/verify-otp`
Verify the OTP and activate the account.

**Request** `{ "email": "john@example.com", "code": "123456" }`
**Response `200`** — message `"otp verification success"`, `data: null`.
Errors: `403` too many failed attempts (lockout), `400/401` invalid/expired code.

### POST `/auth/refresh`
Rotate the token pair using a valid refresh token. Tokens issued before the
last password change are rejected.

**Request** `{ "refresh_token": "<refresh_jwt>" }`
**Response `200`** — message `"token refreshed"`, `data` is the same shape as login (new `token` + `refresh_token`).
Errors: `401` invalid/expired/revoked refresh token.

### POST `/auth/logout`
Revoke the supplied refresh token. If `access_token` is also supplied, its
jti lands on a Redis deny-list for the token's remaining lifetime, so the
access token stops working immediately as well.

**Request** `{ "refresh_token": "<refresh_jwt>", "access_token": "<access_jwt>" }` (`access_token` optional)
**Response `200`** — message `"logout success"`, `data: null`.

### POST `/auth/password/forgot`
Begin a password reset. Always returns 200 (does not reveal whether the email exists).

**Request** `{ "email": "john@example.com" }`
**Response `200`** — message `"if the email is registered, a reset code has been sent"`, `data: null`.
A 6-digit OTP is sent via GeregeCloud Verify to the email.

### POST `/auth/password/reset`
Complete a password reset with the OTP code emailed by the forgot-password flow.

**Request** `{ "email": "john@example.com", "code": "123456", "new_password": "N3w!Str0ngPass" }`
**Response `200`** — message `"password reset"`, `data: null`.
Errors: `401` reset code is invalid or expired, `422` validation.

### PUT `/auth/password/change` 🔒
Change the password for the authenticated user. Requires `Authorization: Bearer`.

**Request**
```json
{ "current_password": "Str0ng!Passw0rd", "new_password": "N3w!Str0ngPass" }
```
**Response `200`** — message `"password changed"`, `data: null`.
Errors: `401` wrong current password / missing token, `422` validation.

---

## Users

### GET `/users/me` 🔒
Return the authenticated user's profile. Requires `Authorization: Bearer`.

**Response `200`**
```json
{ "status": true, "message": "user data fetched successfully", "data": { "user": {
  "id": "…", "username": "johndoe", "email": "john@example.com", "role_id": 2,
  "created_at": "…", "updated_at": null } }, "request_id": "…" }
```
Errors: `401` missing/invalid token.

---

## AI (Gemini pipeline) 🔒

All `/ai/*` endpoints require a bearer token and share a dedicated rate
limit (~20 req/min per IP). They are no-ops returning 500 until
`GEMINI_API_KEY` is configured. The assistant runs on a layered system
prompt — hardcoded guardrails + an admin-configurable **scope** (the
assistant refuses anything outside it) + optional **instructions** — and
grounds platform answers in the `ai_knowledge` table via its
`search_knowledge` tool.

### POST `/ai/chat` 🔒
Chat with the assistant. Send text, voice (base64 audio the model
understands directly), or both. The conversation is stateless — pass prior
turns in `history`.

**Request**
```json
{ "message": "what time is it?",
  "audio": { "mime": "audio/webm", "data": "<base64>" },
  "history": [ { "role": "user", "text": "…" }, { "role": "model", "text": "…" } ] }
```
| Field | Rules |
|-------|-------|
| `message` | optional (required if no `audio`), ≤ 4000 chars |
| `audio` | optional; `mime` ∈ webm/ogg/wav/mpeg/mp3/mp4/m4a/aac/flac, `data` base64 ≤ ~700 KB |
| `history` | optional, ≤ 20 turns |

**Response `200`**
```json
{ "status": true, "message": "ai reply generated", "data": {
  "reply": "Одоо 12:30 цаг болж байна.",
  "steps": [ { "tool": "get_server_time", "args": {}, "result": { } } ],
  "degraded": false }, "request_id": "…" }
```
`steps` lists the function calls the model executed (pipeline trace). When
Gemini is temporarily unavailable the endpoint still returns `200` with a
Mongolian fallback `reply` and `degraded: true`.

### POST `/ai/stt` 🔒
Speech-to-text. **Request** `{ "audio": { "mime": "audio/webm", "data": "<base64>" } }`
**Response `200`** — `data: { "text": "…" }` (empty when no speech detected).

### POST `/ai/tts` 🔒
Text-to-speech. **Request** `{ "text": "Сайн байна уу", "voice": "Kore" }` (`voice` optional)
**Response `200`** — `data: { "mime": "audio/wav", "data": "<base64 WAV>" }` — playable directly in a browser.

### POST `/ai/translate` 🔒
Live translation. Provide `text` **or** `audio` (audio goes through an
internal STT step first); `speak: true` additionally returns a spoken (TTS)
version of the translation. Silent audio chunks return empty fields — the
live-translation UI streams short recorded segments here.

**Request** `{ "audio": { … }, "target_lang": "en", "speak": false }`
(`target_lang`: required, e.g. `mn|en|ru|zh|ja|ko|de`)
**Response `200`** — `data: { "source_text": "Сайн уу", "translated": "Hello", "audio": { … } }`.

### GET `/admin/ai/prompts` · PUT `/admin/ai/prompts/{key}` 🔒
Admin (requires the `settings.manage` permission): list / update the
configurable prompt layers. `key` ∈ `scope | instructions`; body
`{ "content": "…" }` (≤ 4000 chars, empty allowed). Changes take effect
immediately (server-side prompt cache is invalidated). The base guardrail
layer is hardcoded and not exposed here.

---

## Operations (no `/api/v1` prefix)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Liveness — always 200 if the process is up |
| GET | `/ready` | Readiness — pings Postgres (pgx pool) + Redis |
| GET | `/metrics` | Prometheus exposition |
| GET | `/swagger/*` | Swagger UI + spec |

---

🔒 = requires `Authorization: Bearer <access_token>`. Regenerate this spec from
handler annotations with `make swag`.

---

**Gerege Template Version 27.0** — Co-developed by the **Gerege Systems Development Team** and **Claude AI**, 2026.
