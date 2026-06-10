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
- **Rate limit:** `/auth/*` is capped at ~5 requests/minute per IP (429 on excess)
- **Body cap:** `/auth/*` bodies are limited to 4 KiB

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
{ "username": "johndoe", "email": "john@example.com", "password": "Str0ng!Passw0rd" }
```
| Field | Rules |
|-------|-------|
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
Revoke the supplied refresh token.

**Request** `{ "refresh_token": "<refresh_jwt>" }`
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
